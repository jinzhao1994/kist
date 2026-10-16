import { useEffect, useState, type DragEvent as ReactDragEvent } from "react";
import { useDropzone, type DropzoneOptions } from "react-dropzone";

// The list is only part of the page. Forward drops on its surrounding content
// area through the same intake checks, without intercepting sidebar navigation.
export function useUploadDropzone(options: DropzoneOptions) {
  const [paste, setPaste] = useState<{
    files: File[];
    event: ClipboardEvent;
  }>();
  useEffect(() => {
    if (options.disabled || paste) return;
    const onPaste = (event: ClipboardEvent) => {
      const target = event.target;
      const root = drop.rootRef.current;
      const main = root?.closest("main") || root;
      if (
        !main ||
        !(target instanceof Element) ||
        document.querySelector('[role="dialog"]')
      )
        return;
      if (
        target.closest(
          'input, textarea, [contenteditable]:not([contenteditable="false"])',
        )
      )
        return;
      if (target !== document.body && !main.contains(target)) return;
      const files = Array.from(event.clipboardData?.files || []);
      if (!files.length) return;
      event.preventDefault();
      setPaste({ files, event });
    };
    document.addEventListener("paste", onPaste);
    return () => document.removeEventListener("paste", onPaste);
  }, [options.disabled, paste]);
  const drop = useDropzone({
    ...options,
    noPaste: true,
    disabled: options.disabled || !!paste,
    onDrop: (files, rejected, event) => {
      if (document.querySelector('[role="dialog"]')) return;
      options.onDrop?.(files, rejected, event);
    },
  });
  useEffect(() => {
    const main = drop.rootRef.current?.closest("main");
    if (
      !main ||
      options.disabled ||
      !drop.isDragGlobal ||
      document.querySelector('[role="dialog"]')
    )
      return;
    main.classList.add("upload-drag-active");
    return () => main.classList.remove("upload-drag-active");
  }, [drop.rootRef, drop.isDragGlobal, options.disabled]);
  const handlers = drop.getRootProps();
  useEffect(() => {
    const root = drop.rootRef.current;
    const main = root?.closest("main");
    if (!main || options.disabled) return;
    const blocked = () => !!document.querySelector('[role="dialog"]');
    const forward = (event: DragEvent) => {
      if (!(event.target instanceof Node) || root.contains(event.target))
        return;
      if (
        blocked() ||
        !Array.from(event.dataTransfer?.types || []).includes("Files")
      )
        return;
      // react-dropzone supports native drag events as well as React events.
      const handler = {
        dragenter: handlers.onDragEnter,
        dragover: handlers.onDragOver,
        dragleave: handlers.onDragLeave,
        drop: handlers.onDrop,
      }[event.type];
      handler?.(event as unknown as ReactDragEvent<HTMLElement>);
    };
    main.addEventListener("dragenter", forward);
    main.addEventListener("dragover", forward);
    main.addEventListener("dragleave", forward);
    main.addEventListener("drop", forward);
    return () => {
      main.removeEventListener("dragenter", forward);
      main.removeEventListener("dragover", forward);
      main.removeEventListener("dragleave", forward);
      main.removeEventListener("drop", forward);
    };
  }, [
    drop.rootRef,
    options.disabled,
    handlers.onDragEnter,
    handlers.onDragOver,
    handlers.onDragLeave,
    handlers.onDrop,
  ]);
  return {
    ...drop,
    pasteFiles: paste?.files,
    cancelPaste: () => setPaste(undefined),
    confirmPaste: () => {
      if (!paste || options.disabled) return;
      setPaste(undefined);
      options.onDrop?.(paste.files, [], paste.event);
    },
  };
}
