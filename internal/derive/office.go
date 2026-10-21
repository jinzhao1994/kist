package derive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const OfficePDF = "office_pdf_v1"
const OfficeThumbnail = "office_thumb_512_v1"
const SlidesPDF = "slides_pdf_v1"
const SlidesThumbnail = "slides_thumb_512_v1"
const maxOfficeBytes int64 = 128 << 20
const maxOfficeWorkBytes int64 = 512 << 20

// The extension only selects an attempt in the UI. The renderer accepts an
// OOXML package or a compound file forced through the requested family importer.
func officeFormat(ctx context.Context, input string, slides bool) (string, string, error) {
	f, err := os.Open(input)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", "", err
	}
	if st.Size() > maxOfficeBytes {
		return "", "", fail("limit_exceeded", "Office input exceeds 128 MiB")
	}
	var header [8]byte
	_, _ = f.ReadAt(header[:], 0)
	if bytes.Equal(header[:], []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		if slides {
			return ".ppt", "MS PowerPoint 97", nil
		}
		return ".doc", "MS Word 97", nil
	}
	z, err := zip.NewReader(f, st.Size())
	if err != nil {
		return "", "", fail("unsupported", "not an Office package or compound document")
	}
	if len(z.File) > 10000 {
		return "", "", fail("limit_exceeded", "too many Office package entries")
	}
	var total uint64
	document, types := false, false
	for _, entry := range z.File {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		if entry.UncompressedSize64 > uint64(maxOfficeWorkBytes)-total {
			return "", "", fail("limit_exceeded", "expanded Office package exceeds limit")
		}
		total += entry.UncompressedSize64
		switch entry.Name {
		case "word/document.xml":
			if !slides {
				document = true
			}
		case "ppt/presentation.xml":
			if slides {
				document = true
			}
		case "[Content_Types].xml":
			types = true
		}
		if strings.HasSuffix(strings.ToLower(entry.Name), "vbaproject.bin") {
			return "", "", fail("unsupported", "macro-enabled Office packages are not supported")
		}
		if strings.HasSuffix(entry.Name, ".rels") {
			if entry.UncompressedSize64 > 4<<20 {
				return "", "", fail("limit_exceeded", "Office relationships exceed limit")
			}
			r, e := entry.Open()
			if e != nil {
				return "", "", e
			}
			e = checkOfficeRelationships(ctx, r)
			r.Close()
			if e != nil {
				return "", "", e
			}
		}
	}
	if !document || !types {
		return "", "", fail("unsupported", "ZIP does not contain the requested Office document")
	}
	if slides {
		return ".pptx", "Impress MS PowerPoint 2007 XML", nil
	}
	return ".docx", "Office Open XML Text", nil
}

func checkOfficeRelationships(ctx context.Context, r io.Reader) error {
	decoder := xml.NewDecoder(io.LimitReader(contextReader{ctx, r}, (4<<20)+1))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return &Failure{"decode_failed", err}
		}
		if e, ok := token.(xml.StartElement); ok && e.Name.Local == "Relationship" {
			var mode, kind string
			for _, a := range e.Attr {
				switch a.Name.Local {
				case "TargetMode":
					mode = a.Value
				case "Type":
					kind = a.Value
				}
			}
			// Hyperlinks are retained in the PDF, but linked images, templates and
			// embedded external objects must never be fetched during conversion.
			if strings.EqualFold(mode, "External") && !strings.HasSuffix(kind, "/hyperlink") {
				return fail("unsupported", "Office document contains external resources")
			}
		}
	}
}

const officeProfile = `<?xml version="1.0" encoding="UTF-8"?>
<oor:items xmlns:oor="http://openoffice.org/2001/registry">
<item oor:path="/org.openoffice.Office.Common/Security/Scripting"><prop oor:name="DisableMacrosExecution" oor:op="fuse" oor:finalized="true"><value>true</value></prop><prop oor:name="MacroSecurityLevel" oor:op="fuse"><value>3</value></prop></item>
<item oor:path="/org.openoffice.Office.Writer/Content/Update"><prop oor:name="Link" oor:op="fuse" oor:finalized="true"><value>2</value></prop><prop oor:name="Field" oor:op="fuse"><value>false</value></prop><prop oor:name="Chart" oor:op="fuse"><value>false</value></prop></item>
</oor:items>`

func (r Renderer) office(ctx context.Context, input, output string, spec Recipe) (Result, error) {
	var result Result
	slides := spec.Name == SlidesPDF || spec.Name == SlidesThumbnail
	thumbnail := spec.Name == OfficeThumbnail || spec.Name == SlidesThumbnail
	extension, filter, err := officeFormat(ctx, input, slides)
	if err != nil {
		return result, err
	}
	executable, err := tool(r.LibreOffice, "libreoffice")
	if err != nil && r.LibreOffice == "" {
		executable, err = tool("", "soffice")
	}
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp(filepath.Dir(output), ".render-office-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.Abs(dir)
	if err != nil {
		return result, err
	}
	profile := filepath.Join(dir, "profile")
	if err = os.MkdirAll(filepath.Join(profile, "user"), 0700); err != nil {
		return result, err
	}
	if err = os.WriteFile(filepath.Join(profile, "user", "registrymodifications.xcu"), []byte(officeProfile), 0600); err != nil {
		return result, err
	}
	source := filepath.Join(dir, "document"+extension)
	if err = copyOfficeFile(ctx, input, source, maxOfficeBytes); err != nil {
		return result, err
	}
	export := `pdf:writer_pdf_Export:{"IsSkipEmptyPages":{"type":"boolean","value":"false"}}`
	if thumbnail {
		export = `pdf:writer_pdf_Export:{"PageRange":{"type":"string","value":"1"}}`
	}
	if slides {
		export = strings.Replace(export, "writer_pdf_Export", "impress_pdf_Export", 1)
	}
	profilePath := filepath.ToSlash(profile)
	if filepath.VolumeName(profile) != "" {
		profilePath = "/" + profilePath
	}
	profileURL := (&url.URL{Scheme: "file", Path: profilePath}).String()
	cmd := exec.CommandContext(ctx, executable, "-env:UserInstallation="+profileURL, "--headless", "--nologo", "--nodefault", "--norestore", "--infilter="+filter, "--convert-to", export, "--outdir", dir, source)
	cmd.Env = append(os.Environ(), "SAL_USE_VCLPLUGIN=svp", "TMPDIR="+dir, "TMP="+dir, "TEMP="+dir)
	cmd.Dir = dir
	log := &limitedBuffer{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = log, log
	cmd.WaitDelay = time.Second
	officeProcess(ctx, cmd)
	// Bound scratch use while the process is alive, including its profile and
	// temporary PDF. Cancellation kills the converter's process tree.
	done, watched := make(chan struct{}), make(chan struct{})
	oversized := false
	go func() {
		defer close(watched)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				var size int64
				_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
					if e == nil && !d.IsDir() {
						if st, e := d.Info(); e == nil {
							size += st.Size()
						}
					}
					if size > maxOfficeWorkBytes {
						return io.EOF
					}
					return nil
				})
				if size > maxOfficeWorkBytes {
					oversized = true
					cancel()
					return
				}
			}
		}
	}()
	err = cmd.Run()
	close(done)
	<-watched
	if oversized {
		return result, fail("limit_exceeded", "Office conversion exceeds scratch limit")
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err != nil {
		return result, &Failure{"decode_failed", err}
	}
	pdf := filepath.Join(dir, "document.pdf")
	f, err := os.Open(pdf)
	if err != nil {
		return result, fail("decode_failed", "Office conversion produced no PDF")
	}
	var magic [5]byte
	_, err = io.ReadFull(f, magic[:])
	st, statErr := f.Stat()
	f.Close()
	if err != nil || statErr != nil || string(magic[:]) != "%PDF-" {
		return result, fail("decode_failed", "Office conversion produced invalid PDF")
	}
	if st.Size() > maxOfficeBytes {
		return result, fail("limit_exceeded", "converted PDF exceeds 128 MiB")
	}
	if thumbnail {
		result, err = r.pdf(ctx, pdf, output, spec)
	} else {
		err = copyOfficeFile(ctx, pdf, output, maxOfficeBytes)
	}
	result.Engine = "libreoffice"
	return result, err
}

func copyOfficeFile(ctx context.Context, source, destination string, limit int64) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer dst.Close()
	n, err := io.Copy(dst, io.LimitReader(contextReader{ctx, src}, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fail("limit_exceeded", "Office conversion file exceeds limit")
	}
	if err = dst.Sync(); err != nil {
		return err
	}
	return dst.Close()
}
