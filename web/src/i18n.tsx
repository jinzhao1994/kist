import { useDirection } from "@mantine/core";
import {
  createContext,
  useContext,
  useLayoutEffect,
  useState,
  type ReactNode,
} from "react";

export const en = {
  clientSelectOffline: "Select files for offline use",
  clientSelectLoaded: "Select loaded items ({count})",
  clientSelectedCount: "{count} selected",
  clientBatchHint:
    "Each selected file or file set is retained separately. File sets include their configured members. Manage retention in Offline client → Cache.",
  clientBatchProgress:
    "{retained} of {total} retained · {prepared} of {total} prepared",
  clientBatchInterrupted:
    "Stopped. Saved selections and downloaded content are kept; check and download again to continue.",
  clientBatchChanged:
    "This selection changed during preparation. Check and download again for its current members.",
  clientRetainedItems: "Retained offline",
  clientRetainedEmpty:
    "No offline selections yet. Use Keep offline in a file or file set menu.",
  clientRetainedCount: "{files} files · {filesets} file sets",
  clientRetainedDetails: "Check availability",
  clientKeepOffline: "Keep offline",
  clientKeepFilesetHint:
    "Retain every configured member, including nested file sets and Live Photo video.",
  clientKeepFileHint: "Retain this original on the laptop for offline use.",
  clientRetentionSaved: "Offline retention is enabled.",
  clientOfflinePreviews: "{ready} of {total} previews available.",
  clientOfflineReady:
    "All required originals and previews are available offline.",
  clientConflictOrdinaryFolder: "Ordinary folder",
  clientConflictReview: "Review local change",
  clientConflictReviewHint:
    "Compare your original change with the current state on this laptop. Saving creates a new local intention; synchronization will check the NAS again.",
  clientConflictUnsupported:
    "This change requires its own domain editor. You can return and keep the NAS result; the original local intention remains in history.",
  clientConflictBefore: "Before your change",
  clientConflictCurrent: "Current value",
  clientConflictChosen: "Value to submit",
  clientConflictSave: "Save revised change",
  clientOfflineMissing:
    "{ready} of {total} originals available. More content still needs preparing.",
  clientPrepareAgain: "Check and download",
  clientKeepAndPrepare: "Retain and download",
  clientStopKeeping: "Stop retaining offline",
  clientPreparing:
    "Preparing offline content\u2026 Progress is retained if interrupted.",
  clientRetentionScope:
    "Readiness uses the current local catalog. Stopping retention does not delete archive files; cached copies become eligible for cleanup.",

  clientInterrupted:
    "Network request stopped. Local progress is retained; sync again to confirm the result.",
  clientTitle: "Offline client",
  clientHistory: "Local operations",
  clientHistoryHint:
    "Local operations keep their original identity. NAS history contains only accepted operations.",
  clientHistoryBack: "Back to local operations",
  clientHistoryEmpty: "No local operations yet.",
  clientHistoryPending: "Pending sync",
  clientHistoryAwaiting: "Awaiting confirmation",
  clientHistoryConflict: "Conflict",
  clientHistoryAccepted: "Accepted by NAS",
  clientHistoryDiscarded: "Discarded",
  clientHistoryReplaced: "Replaced",
  clientFormalOperation: "NAS operation #{id}",
  clientHistoryUndoTarget: "Undo target",
  clientHistoryGroup: "Operation group",
  clientHistoryRelated: "View related local operation",
  clientHistoryReplacement: "View replacement",
  clientSync: "Sync",
  clientCache: "Cache",
  clientConnection: "Connection",
  clientNotConnected: "Not connected to a NAS",
  clientCatalogComplete:
    "Search includes all metadata received in the last completed sync. Originals are cached separately.",
  clientCatalogIncomplete:
    "The catalog has not finished syncing. Browsing and search may be incomplete.",
  clientSyncNow: "Sync now",
  clientSyncComplete: "Sync complete",
  clientSyncConflict:
    "Sync paused for a conflict. Your local change has been retained.",
  clientPending: "Pending changes · {count}",
  clientNoPending: "No pending changes",
  clientAwaitingReceipt:
    "The last submission is awaiting confirmation. Sync again to check its result.",
  clientDiscardConfirm:
    "I want to discard this local change and keep the NAS state.",
  clientDiscard: "Discard local change",
  clientResolved: "Decision saved. Sync again to continue.",
  clientCacheUsed: "Cached content: {size}",
  clientPinnedUsed: "Retained offline: {size}",
  clientProtectedUsed:
    "Protected local originals: {originals} · Local history: {journal}. These are excluded from automatic cache cleanup.",
  clientCacheOverLimit:
    "Retained or active content exceeds the cache limit. Increase the limit or reduce offline retention.",
  clientPrune: "Clean expired cache",
  clientCachePruned: "Cache cleanup complete",
  clientServer: "NAS address",
  clientToken: "Sync credential",
  clientAllowHTTP: "Allow unencrypted HTTP outside this computer",
  clientConnectionHint:
    "Use a sync credential created on your NAS. This client remains bound to the same library.",
  clientConnect: "Verify and connect",
  clientMaxAge: "Maximum cache age",
  clientMaxAgeHint: "Measured since last access, for example 720h for 30 days.",
  clientMaxSize: "Maximum cache size (MiB)",

  filesetLabel: "File set",
  filesetId: "File set ID",
  copyFilesetId: "Copy file set ID",
  filesetConfigHistory: "Combination configuration history",
  filesetPrimaryDetails: "Primary file details and tags",
  filesetCurrentPage: "Page {page}",
  filesetConfigure: "Configure file set",
  filesetType: "File set type",
  filesetPrimary: "Primary file",
  filesetFirstPage: "Automatic · first page",
  filesetPages: "Pages",
  filesetPagesHelp: "About page order and membership",
  filesetPageOrderHint:
    "Drag to reorder, or focus a handle and use the arrow keys.",
  filesetDragPage: "Reorder {name}",
  filesetRemovePage: "Remove {name} from pages",
  filesetReordered: "Moved to page {page}",
  filesetAddPage: "Add page",
  filesetChoosePage: "Choose a file from this folder…",
  filesetNoPages: "No pages yet. Add a file below.",
  filesetExcludedHint:
    "Files outside this list remain in the folder. Removing a page does not delete its file.",
  filesetAdvanced: "Advanced settings",
  filesetAutomatic: "Automatic (by role / first page)",
  filesetLayout: "Filesystem layout",
  filesetFlat: "Shared prefix, same directory",
  filesetDirectory: "Keep directory structure",
  filesetOrderHint:
    "The list defines roles and page order. Other files remain accessible.",
  filesetMoveUp: "Move page up",
  filesetMoveDown: "Move page down",
  filesetRemoveMember: "Remove role",
  filesetRole: "Role",
  filesetLanguage: "Subtitle language",
  filesetDefaultSubtitle: "Default subtitle",
  filesetCancel: "Convert to ordinary folder",
  filesetPreview: "File set preview",
  filesetPageTools: "Page navigation",
  noTags: "No tags",
  filesetLive: "LIVE",
  filesetStill: "Show photo",
  filesetMotion: "Play motion",
  filesetSubtitleError:
    "Cannot load subtitles. Use UTF-8 SRT or WebVTT (up to 4 MiB).",
  filesetIncomplete:
    "This file set is incomplete. Configure its members or upload the missing files.",
  filesetView: "View file set",
  filesetShowMembers: "Show all files",
  filesetHideMembers: "Hide files",
  filesetIncludeMembers: "Include files within sets",
  filesetMemberScope: "Files within sets",
  filesetMembersIncluded: "Include",
  filesetMembersExcluded: "Exclude",
  fileset_live_photo: "Live Photo",
  fileset_video_subtitles: "Video and subtitles",
  fileset_document_pages: "Multi-page document",
  fileset_role_image: "Photo",
  fileset_role_video: "Video",
  fileset_role_subtitle: "Subtitle",
  fileset_role_page: "Page",
  filesetPrevious: "Previous page",
  filesetNext: "Next page",
  action_set_fileset: "Configure file set",
  action_delete_fileset_entry: "Trash file set member",
  action_restore_fileset_entry: "Restore file set member",

  setCover: "Set as box cover",
  removeCover: "Remove cover",
  mapZoomIn: "Zoom in",
  mapZoomOut: "Zoom out",
  mapAttribution: "Map attribution",
  mapLoading: "Loading map…",
  mapUnavailable: "Map background unavailable",
  action_upload_files: "Upload and replace files",
  action_undo_upload_files: "Undo file upload",
  uploadConflictTitle: "Files already exist",
  uploadConflictHint:
    "These names already exist in this folder. Apply one choice to all conflicts: overwrite keeps the previous content in history; keep both adds a number to the new names.",
  uploadFolderConflict:
    "A name belongs to a folder. Choose keep both to avoid replacing the folder.",
  uploadOverwrite: "Overwrite",
  uploadKeepBoth: "Keep both",
  searchTags: "Search tags",
  searchOptions: "Search options",
  tagResultCounts:
    "{boxes, plural, one {# box} other {# boxes}} · {files, plural, one {# file} other {# files}}",
  noMatchingBoxes: "No matching boxes",
  noTaggedFilesHint:
    "Files appear here when assigned this tag or one of its subtags.",
  expandTag: "Expand {name}",
  collapseTag: "Collapse {name}",
  folderType: "Folder",
  folders: "Folders",
  createFolder: "New folder",
  emptyFolder: "No files or folders yet",
  emptyFolderUploadHint: "Drop files here, or click Upload files.",
  uploadToFolder: "Files will be added to the current folder.",
  folderPath: "Folder path",
  archiveRoot: "Root",
  boxRoot: "Box",
  returnToBoxRoot: "Return to box root",
  searchWholeBox: "Search scope: current folder and subfolders",
  historyBoxTarget: "box {target}",
  historyFileTarget: "file {target}",
  historyNamed: "“{name}”",
  historyBatch: "“{name}” and others ({count} items)",
  historyTagAdded: "Add tag “{tag}” to {target}",
  historyTagRemoved: "Remove tag “{tag}” from {target}",
  historyRenamed: "“{before}” → “{after}”",
  historyCurrentNames: "Current names",
  historyCurrentNamesHint:
    "This older operation stores object IDs; referenced objects are shown using their current names, or IDs if unavailable.",
  historySummaryUnavailable: "Summary unavailable",
  historyAcceptedAt: "Accepted by NAS",
  history: "History",
  trash: "Recycle bin",
  historyEmpty: "No operations here yet.",
  trashHint: "Deleted items are retained and can be restored.",
  trashAllScopes: "All records",
  trashEmpty: "No deleted items in this scope",
  restoreConfirm:
    "Restore all items from this deletion? Later changes may prevent restoration.",
  historyScope: "History scope",
  allHistory: "All operations",
  operation: "Operation #{id}",
  undo: "Undo",
  restore: "Restore",
  undoConfirm:
    "Append an inverse operation? This applies to every item in this operation and may be refused if later changes conflict.",
  undoneBy: "Reverted by #{id}",
  reverts: "Reverts #{id}",
  blockedBy: "Blocked by operation #{id}",
  viewOperation: "View operation",
  viewHistoricalBox: "View box at this point",
  historicalState: "Read-only state after operation #{id}",
  versionHistory: "File versions",
  compareVersions: "Compare versions",
  beforeVersion: "Earlier operation",
  afterVersion: "Later operation",
  compare: "Compare",
  field: "Field",
  notPresent: "Not present at this point",
  noDifferences: "No differences between these states.",
  changedFields: "Changed fields are highlighted.",
  viewBeforeDeletion: "View before deletion",
  affectedFiles: "Affected files",
  affectedBoxes: "Affected boxes",
  operationData: "Recorded operation data",
  blobKey: "Content fingerprint",
  updated: "Updated",
  origin: "Origin",
  action_create_box: "Create box",
  action_delete_box: "Delete box",
  action_delete_box_contents: "Move box to trash",
  action_rename_entry: "Rename file or folder",
  action_delete_entry_contents: "Move file or folder to trash",
  action_restore_entry_contents: "Restore file or folder",
  trashEntry: "Move to trash",
  trashEntryConfirm: "Move “{name}” to trash?",
  trashFolderContents: "Includes {files} files and {folders} subfolders.",
  trashEntryRestoreHint: "You can restore it from Trash.",
  action_restore_box_contents: "Restore box",
  moveBoxToTrash: "Move to trash",
  trashBoxConfirm:
    "Move “{name}” and all {count, number} files to trash? Folders, tags and metadata will be restored together. Original files are retained.",
  trashBoxChanged:
    "The box may have changed. Close this dialog and try again to review its current contents.",
  action_update_box: "Edit box",
  savedSearches: "Favorites",
  saveSearch: "Save this search",
  editSavedSearch: "Edit saved search",
  replaceSavedSearch: "Use current search conditions",
  replaceSearchHint:
    "Saving replaces the saved scope, conditions and sorting with the current search.",
  removeSavedSearch: "Remove saved search",
  removeSavedSearchHint:
    "Remove this saved search? Files and boxes are unchanged. You can undo this in history.",
  savedSearchLiveHint: "Results update as your library changes",
  savedSearchMissing:
    "This search references a missing tag, box or folder, or an unsupported definition. Its conditions have been preserved. Restore the missing item or replace the saved conditions.",
  action_create_saved_search: "Save search",
  action_update_saved_search: "Update saved search",
  action_delete_saved_search: "Remove saved search",
  action_create_tag: "Create tag",
  action_delete_tag: "Delete tag",
  action_rename_tag: "Rename tag",
  action_move_tag: "Move tag",
  action_tag_boxes: "Tag boxes",
  action_untag_boxes: "Untag boxes",
  action_create_folder: "Create folder",
  action_delete_folder: "Delete folder",
  action_import_files: "Import files",
  action_delete_files: "Delete files",
  action_extract_metadata: "Extract metadata",
  action_set_metadata: "Edit metadata",
  action_tag_files: "Tag files",
  action_untag_files: "Untag files",
  action_request_receipt: "No-change receipt",

  mediaNotCached: "This preview is not cached. Connect to your NAS and retry.",
  mediaCacheQuota:
    "There is not enough cache space for this preview. Adjust offline retention or the cache limit and retry.",
  mediaCacheMetadata:
    "The preview cache index needs rebuilding in cache settings.",
  mediaCacheCorrupt:
    "Preview cache verification failed. Rebuild the cache index and download it again.",
  mediaSyncVersion:
    "This NAS does not support compatible preview transfers. Update it and retry.",
  mediaQueued: "Waiting to generate preview…",
  mediaRunning: "Generating preview…",
  mediaDependency:
    "Preview tools are missing. Video needs FFmpeg; HEIC needs libheif with an HEVC decoder; PDF needs Poppler; Word/PowerPoint need LibreOffice Writer/Impress (and Poppler for thumbnails). Will retry automatically.",
  mediaFailed:
    "Preview generation failed. The original is still available to download.",
  mediaCancelled: "Preview generation cancelled.",
  mediaLimit:
    "This file exceeds the preview size or duration limit. Download the original to view it.",
  mediaTimeout:
    "Preview generation timed out. You can retry or download the original.",
  mediaRequestFailed: "Could not read or update preview status. Please retry.",
  files: "Files",
  saving: "Saving…",
  newOperation: "Start a new operation",
  invalidInput: "Check the entered values and start a corrected operation.",
  conflict:
    "The archive changed or this operation conflicts with existing data. Refresh and try again.",
  tagCycle: "A tag cannot be moved into its own subtree.",
  deleteTagConfirm: "Remove this tag?",
  deleteTagTitle: "Delete tag",
  openTagPage: "Open tag page",
  viewTaggedBoxes: "View boxes",
  viewTaggedFiles: "View files",
  deleteNamedTagConfirm: "Delete the tag “{name}”?",
  uploadTooLarge:
    "The batch exceeds the configured upload limits. Choose fewer or smaller files.",
  appName: "kist",
  archive: "Your archive",
  boxes: "Boxes",
  allFiles: "All files",
  tags: "Tags",
  allTags: "Tag directory",
  settings: "Display settings",
  primaryColor: "Primary color",
  gray: "Gray",
  green: "Green",
  yellow: "Yellow",
  pink: "Pink",
  blue: "Blue",
  orange: "Orange",
  colorMode: "Appearance",
  modeSystem: "Follow system",
  modeLight: "Light",
  modeDark: "Dark",
  language: "Language",
  formatLocale: "Regional format",
  timezone: "Time zone",
  direction: "Reading direction",
  auto: "Automatic",
  ltr: "Left to right",
  rtl: "Right to left",
  close: "Close",
  save: "Save",
  cancel: "Cancel",
  loading: "Loading…",
  retry: "Try again",
  error: "Something went wrong. Please try again.",
  errorDetails: "Something went wrong. {message} (error: {code})",
  createBox: "New box",
  emptyBoxes: "A place for the things you want to keep.",
  emptyBoxesHint:
    "Create a box for a trip, an experience, or a batch of documents.",
  name: "Name",
  type: "Type",
  description: "Description",
  startAt: "Start date",
  endAt: "End date",
  today: "Today",
  useDateYear: "Use {year}",
  useDateMonth: "Use {year}-{month}",
  nextMonth: "Next month",
  previousMonth: "Previous month",
  nextYear: "Next year",
  previousYear: "Previous year",
  nextDecade: "Next decade",
  previousDecade: "Previous decade",
  chooseMonth: "Choose month",
  chooseYear: "Choose year",
  dateHint: "Year, month or date (for example, 2026 or 2026-10-01)",
  editBox: "Edit box",
  upload: "Add files",
  pasteUploadTitle: "Upload pasted files?",
  pasteUploadHint:
    "These files came from your clipboard. Nothing is uploaded until you confirm.",
  search: "Search filenames",
  noFiles: "No files here yet",
  noResults: "No matching files",
  importHint: "Drop files here, or choose files to add to this box.",
  chooseFiles: "Choose files",
  uploadFiles: "Upload files",
  openItem: "Open",
  download: "Download original",
  openPDF: "Open PDF in new tab",
  openImage: "Open image in new tab",
  sheetView: "Table view",
  sheetTable: "Table",
  sheetFailed:
    "Unable to read this spreadsheet. You can still view CSV source or download the original file.",
  sheetLimit:
    "This spreadsheet exceeds the preview size or complexity limit. Download the original file to open it.",
  sheetMissingFormula: "No saved result",
  sheetPreview: "Spreadsheet preview",
  sheetReader: "Spreadsheet reader",
  sheetWorksheet: "Worksheet",
  sheetSearch: "Search this page",
  sheetCopied: "Selection copied",
  sheetCopyFailed: "Copy failed. Try Ctrl/Cmd+C in the table.",
  sheetCopy: "Copy selection",
  sheetCachedValues: "Saved formula results; no recalculation",
  sheetLegacy: "Legacy XLS: some formatting may differ",
  sheetTruncated: "Long cell text was shortened",
  sheetMatches: "matches on this page",
  sheetCells: "Table cells",
  sheetEmpty: "This page has no cells",
  sheetPreviousRows: "Previous rows",
  sheetNextRows: "Next rows",
  sheetPreviousColumns: "Previous columns",
  sheetNextColumns: "Next columns",
  markdownMode: "Markdown view",
  markdownPreview: "Preview",
  markdownSource: "Source",
  markdownMissing: "Referenced file is unavailable",
  markdownExternalImage: "External image",
  markdownImageBlocked: "Not loaded automatically",
  markdownOpenImage: "Open image",
  textPreview: "Text preview",
  textReader: "Text reader",
  textEncoding: "Text encoding",
  textEncodingAuto: "Auto encoding",
  textLoadFailed: "Could not load text.",
  textEncodingUnknown:
    "Cannot preview this text with the supported encodings (UTF-8, UTF-16, GBK/GB18030). It may use another encoding or contain invalid bytes. You can download the original.",
  emptyText: "Empty text file",
  plainText: "Plain text",
  textWrap: "Wrap lines",
  textSearch: "Search text",
  textNoMatches: "No matches in the loaded text.",
  textTruncated:
    "Showing the beginning of this file (up to 1 MiB). Search covers only the loaded text. Download the original to read the complete file.",
  imageLoadFailed: "Could not load the original image.",
  pdfZoomIn: "Zoom in",
  pdfZoomOut: "Zoom out",
  pdfTools: "PDF tools",
  pdfPage: "Page number",
  pdfPages: "PDF pages",
  pdfPrevious: "Previous page",
  pdfNext: "Next page",
  pdfFitWidth: "Fit width",
  pdfSearch: "Search PDF text",
  pdfPreviousMatch: "Previous match",
  pdfNextMatch: "Next match",
  pdfUnavailable:
    "Unable to read this PDF. It may be damaged or password-protected. Open in a new tab or download the original.",
  details: "File details",
  basicInfo: "Basic information",
  mediaKind: "Type",
  mimeType: "MIME type",
  openNewTab: "Open in new tab",
  copyLink: "Copy link",
  copyLinkFailed: "Could not copy link",
  uploadHere: "Upload to this folder",
  copyFileId: "Copy file ID",
  copy: "Copy",
  copied: "Copied",
  imageHeight: "Image height",
  imageWidth: "Image width",
  resolutionUnit: "Resolution unit",
  horizontalResolution: "Horizontal resolution",
  verticalResolution: "Vertical resolution",
  userComment: "Comment",
  cameraMake: "Camera make",
  cameraModel: "Camera model",
  software: "Software",
  previewUnavailable:
    "Preview is not available for this format. You can still download the original.",
  image: "Image",
  video: "Video",
  audio: "Audio",
  document: "Document",
  other: "Other",
  unknown: "Not classified yet",
  kind: "File type",
  allTypes: "All types",
  sort: "Sort by",
  captured: "Content time",
  imported: "Import order",
  filename: "Filename",
  ascending: "Ascending",
  descending: "Descending",
  addFilter: "Add filter",
  searchConditions: "Search or add a condition…",
  contains: "contains",
  operator: "Operator",
  chooseOperator: "Choose an operator…",
  executeSearch: "Search",
  chooseFilter: "Choose a condition",
  filterValue: "Type to choose a value…",
  searchText: "Search this text",
  applyFilter: "Apply",
  filterDateHint: "Content date: YYYY, YYYY-MM or YYYY-MM-DD",
  noFilterOptions: "No matching options",
  from: "Start date",
  to: "End date",
  clearFilters: "Clear filters",
  adjustSearchHint: "Try adjusting or clearing the search conditions",
  loadMore: "Load more",
  searchBoxes: "Search box names and descriptions",
  boxDateHint:
    "Activity interval overlaps this range. Use YYYY, YYYY-MM or YYYY-MM-DD.",
  activityDate: "Activity date",
  sameYear: "Same year",
  sameMonth: "Same month",
  sameDay: "Same day",
  sameMoment: "Same time",
  allBoxesShown: "All {count, number} boxes shown",
  gallery: "Gallery",
  galleryNavigation: "Browse files",
  backToResults: "Back to results",
  previousFile: "Previous file",
  nextFile: "Next file",
  singleFile: "Single file",
  displayMode: "Display mode",
  largeIcons: "Large icons",
  mediumIcons: "Medium icons",
  smallIcons: "Small icons",
  listView: "List",
  fileSize: "Size",
  allFoldersShown:
    "{count, plural, one {All # folder shown} other {All # folders shown}}",
  allEntriesShown:
    "All {folders, plural, one {# folder} other {# folders}} and {files, plural, one {# file} other {# files}} shown",
  allFilesShown:
    "{count, plural, one {All # file shown} other {All # files shown}}",
  filters: "Filters",
  tagAll: "All of these tags",
  tagAny: "Any of these tags",
  tagNot: "None of these tags",
  newTag: "New tag",
  rename: "Rename",
  tagActions: "Actions for {name}",
  addChildTag: "Add child tag",
  changeParentTag: "Change parent tag",
  move: "Move",
  remove: "Remove",
  parentTag: "Parent tag",
  rootTag: "No parent",
  manageTags: "Manage tags",
  addTag: "Add tag",
  removeTag: "Remove tag",
  selectTag: "Select a tag",
  created: "Added",
  modified: "Updated",
  capturedAt: "Content time",
  metadata: "Metadata",
  path: "Path",
  fileId: "File ID",
  extractionPending: "Metadata has not been fully read yet.",
  extract: "Read metadata",
  extractionFailed:
    "Some metadata could not be read. Your original files are saved.",
  extracting: "Reading metadata…",
  uploadCancelled:
    "Upload cancelled. Check the result before starting a new upload.",
  uploadFailed:
    "Upload interrupted. Retry with the same files to safely recover the result.",
  uploadComplete: "Files saved",
  uploadElapsed:
    "Upload complete · this attempt {seconds} s · average {speed}/s",
  uploadCurrentStats:
    "Elapsed {seconds} s · average effective transfer {speed}/s",
  uploadTiming:
    "Read/checksum: {hash} s; block requests (including server writes): {transfer} s; final processing: {processing} s. Reading and transfer overlap; stages do not add up to total time. Pauses are excluded.",
  uploading: "Uploading…",
  uploadHashing: "Calculating file checksums…",
  uploadPause: "Pause",
  uploadResume: "Resume",
  uploadReselect: "Select original files again",
  uploadChecking: "Checking saved upload progress…",
  uploadPaused: "Paused. {percent}% confirmed by the server.",
  uploadReselectHint:
    "After refreshing or reopening this page, select the files again so the browser can read them. Select the entire original batch again; uploading resumes automatically after verification, without resending confirmed data.",
  uploadWrongSource:
    "These are not the original files. Select the same batch; names, sizes and timestamps alone are not enough.",
  uploadStorageUnavailable:
    "Upload recovery information cannot be saved or read. Check this site's browser storage, then reload.",
  uploadProtocolError:
    "The server returned incompatible upload progress. Reload and check the result before starting another upload.",
  uploadResumeFailed:
    "Upload interrupted. Confirmed data is retained; resume when the connection is available.",
  uploadExpired:
    "This upload session expired or was removed. Cancel it here, then select the files for a new upload.",
  uploadTargetChanged:
    "The destination or an existing file changed. Cancel this upload, select the files again, and confirm any name conflicts.",
  uploadStorageFull:
    "The server disk is full. Free disk space, then resume this upload.",
  uploadSessionQuota:
    "The server upload staging limit has been reached. Complete or cancel other uploads, then retry.",
  uploadSessionDamaged:
    "This staging session is damaged or uses an unsupported version. Check the result and server log before removing it.",
  uploadBackgroundHint:
    "Keep this page open and the device awake. You do not need to watch it. If the browser suspends it, resume from confirmed data (retained for 7 idle days).",
  uploadProcessing: "Server is verifying and processing files…",
  uploadIntegrityMismatch:
    "File verification failed. Nothing in this batch was archived. Select the original files again and retry.",
  checkResult: "Check result",
  requestPending: "No committed result yet. You can retry this request.",
  projectionDirty:
    "The readable folder view needs repair. Your archive data is saved; run kist relink to rebuild the folder view.",
  loadedFiles: "{count, plural, one {# file loaded} other {# files loaded}}",
  fileCount: "{count, plural, one {# file} other {# files}}",
  uploadLimit: "Up to {count} files and {size} per batch.",
  uploadProgress: "{percent}% uploaded",
  fieldRequired: "Please enter a name.",
  invalidDate: "Use a valid year, month, or date.",
  invalidSettings: "Enter a valid locale and time zone.",
  requestConflict:
    "This request key was already used for different content. Start a new operation.",
  notFound: "This item no longer exists. Refresh the archive.",
  refresh: "Refresh",
  addItems: "Add items",
  moreActions: "More actions",
} as const;
export type MessageKey = keyof typeof en;
export const zh: Record<MessageKey, string> = {
  clientSelectOffline: "选择离线文件",
  clientSelectLoaded: "选择已加载项目（{count}）",
  clientSelectedCount: "已选 {count} 项",
  clientBatchHint:
    "每个选中的文件或组合分别保留，组合包含其配置成员。可在“离线客户端 → 缓存”中管理保留项。",
  clientBatchProgress:
    "已保留 {retained} / {total} 项 · 已准备 {prepared} / {total} 项",
  clientBatchInterrupted:
    "已停止。保存的保留项和已下载内容会留下，再次检查并下载即可继续。",
  clientBatchChanged: "准备期间选择内容发生变化，请重新检查并下载当前成员。",
  clientRetainedItems: "离线保留项",
  clientRetainedEmpty: "尚未保留内容，可在文件或组合的菜单中选择“保留离线”。",
  clientRetainedCount: "{files} 个文件 · {filesets} 个组合",
  clientRetainedDetails: "检查可用状态",
  clientKeepOffline: "保留离线",
  clientKeepFilesetHint:
    "保留组合配置中的全部成员，包括嵌套组合和实况照片的视频。",
  clientKeepFileHint: "将此原文件保留在笔记本上，以便断网时使用。",
  clientRetentionSaved: "已启用离线保留。",
  clientOfflinePreviews: "已准备 {ready} / {total} 个预览。",
  clientOfflineReady: "所需原文件和预览已全部准备好，可离线使用。",
  clientConflictOrdinaryFolder: "普通目录",
  clientConflictReview: "查看本地修改",
  clientConflictReviewHint:
    "对照原始修改和笔记本当前状态。保存会创建新的本地修改，同步时仍会重新核对 NAS。",
  clientConflictUnsupported:
    "此类修改需要专门的编辑方式。可返回并选择保留 NAS 结果，原始本地修改仍保留在历史中。",
  clientConflictBefore: "修改前",
  clientConflictCurrent: "当前值",
  clientConflictChosen: "准备提交的值",
  clientConflictSave: "保存调整后的修改",
  clientOfflineMissing: "已准备 {ready} / {total} 个原文件，仍有内容需要准备。",
  clientPrepareAgain: "检查并下载",
  clientKeepAndPrepare: "保留并下载",
  clientStopKeeping: "取消离线保留",
  clientPreparing: "正在准备离线内容…中断后可继续。",
  clientRetentionScope:
    "准备状态以当前本地目录为准。取消保留不会删除归档文件，缓存副本之后可被自动清理。",

  clientInterrupted: "已停止网络请求，本地进度已保留，请重新同步以核对结果。",
  clientTitle: "离线客户端",
  clientHistory: "本地操作",
  clientHistoryHint: "本地操作保留原始身份；NAS 正式历史只包含已接纳的操作。",
  clientHistoryBack: "返回本地操作",
  clientHistoryEmpty: "暂无本地操作。",
  clientHistoryPending: "待同步",
  clientHistoryAwaiting: "等待确认",
  clientHistoryConflict: "存在冲突",
  clientHistoryAccepted: "NAS 已接纳",
  clientHistoryDiscarded: "已放弃",
  clientHistoryReplaced: "已替换",
  clientFormalOperation: "NAS 操作 #{id}",
  clientHistoryUndoTarget: "撤销目标",
  clientHistoryGroup: "操作组",
  clientHistoryRelated: "查看关联本地操作",
  clientHistoryReplacement: "查看替换方案",
  clientSync: "同步",
  clientCache: "缓存",
  clientConnection: "连接",
  clientNotConnected: "尚未连接 NAS",
  clientCatalogComplete: "检索包含上次完整同步的全部资料信息，原文件另行缓存。",
  clientCatalogIncomplete: "资料目录尚未完成同步，浏览和检索结果可能不完整。",
  clientSyncNow: "立即同步",
  clientSyncComplete: "同步完成",
  clientSyncConflict: "同步遇到冲突，已保留本地修改。",
  clientPending: "待同步修改 · {count}",
  clientNoPending: "暂无待同步修改",
  clientAwaitingReceipt: "上次提交尚待确认，请重新同步以查询结果。",
  clientDiscardConfirm: "我确认放弃这条本地修改，保留 NAS 当前状态。",
  clientDiscard: "放弃本地修改",
  clientResolved: "已保存处理决定，请继续同步。",
  clientCacheUsed: "已缓存内容：{size}",
  clientPinnedUsed: "保留离线内容：{size}",
  clientProtectedUsed:
    "受保护的本地原文件：{originals} · 本地历史：{journal}，均不受自动缓存清理影响。",
  clientCacheOverLimit:
    "保留或正在使用的内容已超过缓存上限，请提高上限或减少离线保留。",
  clientPrune: "清理到期缓存",
  clientCachePruned: "缓存清理完成",
  clientServer: "NAS 地址",
  clientToken: "同步凭据",
  clientAllowHTTP: "允许使用未加密的远程 HTTP 连接",
  clientConnectionHint:
    "使用 NAS 上创建的同步凭据，此客户端始终绑定同一个资料库。",
  clientConnect: "验证并连接",
  clientMaxAge: "最长缓存保留时间",
  clientMaxAgeHint: "从最近访问起计算，例如 720h 表示 30 天。",
  clientMaxSize: "缓存体积上限（MiB）",

  filesetLabel: "文件组合",
  filesetId: "组合 ID",
  copyFilesetId: "复制组合 ID",
  filesetConfigHistory: "组合配置历史",
  filesetPrimaryDetails: "主文件详情与标签",
  filesetCurrentPage: "第 {page} 页",
  filesetConfigure: "配置文件组合",
  filesetType: "组合类型",
  filesetPrimary: "主文件",
  filesetFirstPage: "自动 · 使用第一页",
  filesetPages: "页面",
  filesetPagesHelp: "页面排序与成员说明",
  filesetPageOrderHint: "拖动调整顺序，也可聚焦拖动手柄后使用上下方向键。",
  filesetDragPage: "调整 {name} 的顺序",
  filesetRemovePage: "将 {name} 移出页面",
  filesetReordered: "已移至第 {page} 页",
  filesetAddPage: "添加页面",
  filesetChoosePage: "选择目录中的文件…",
  filesetNoPages: "暂无页面，可在下方添加文件。",
  filesetExcludedHint: "未加入的文件仍保留在目录中。移出页面不会删除文件。",
  filesetAdvanced: "高级设置",
  filesetAutomatic: "自动（按角色／第一页）",
  filesetLayout: "文件系统布局",
  filesetFlat: "共同前缀，放在同一目录",
  filesetDirectory: "保留目录结构",
  filesetOrderHint: "列表决定角色与页序，其他文件仍可访问。",
  filesetMoveUp: "页面上移",
  filesetMoveDown: "页面下移",
  filesetRemoveMember: "移除角色",
  filesetRole: "角色",
  filesetLanguage: "字幕语言",
  filesetDefaultSubtitle: "默认字幕",
  filesetCancel: "转为普通目录",
  filesetPreview: "文件组合预览",
  filesetPageTools: "页面导航",
  noTags: "暂无标签",
  filesetLive: "实况",
  filesetStill: "显示照片",
  filesetMotion: "播放动态",
  filesetSubtitleError: "字幕加载失败。支持 UTF-8 SRT 或 WebVTT，最大 4 MiB。",
  filesetIncomplete: "组合尚不完整，请配置成员或补充缺少的文件。",
  filesetView: "查看组合",
  filesetShowMembers: "查看全部文件",
  filesetHideMembers: "收起文件",
  filesetIncludeMembers: "包含组合内文件",
  filesetMemberScope: "组合内文件",
  filesetMembersIncluded: "包含",
  filesetMembersExcluded: "不包含",
  fileset_live_photo: "Live Photo",
  fileset_video_subtitles: "视频及字幕",
  fileset_document_pages: "多页文档",
  fileset_role_image: "照片",
  fileset_role_video: "视频",
  fileset_role_subtitle: "字幕",
  fileset_role_page: "页面",
  filesetPrevious: "上一页",
  filesetNext: "下一页",
  action_set_fileset: "配置文件组合",
  action_delete_fileset_entry: "回收组合成员",
  action_restore_fileset_entry: "恢复组合成员",

  setCover: "设为归档盒封面",
  removeCover: "移除封面",
  mapZoomIn: "放大地图",
  mapZoomOut: "缩小地图",
  mapAttribution: "地图来源",
  mapLoading: "正在加载地图…",
  mapUnavailable: "地图背景暂不可用",
  folderType: "文件夹",
  folders: "文件夹",
  createFolder: "新建目录",
  emptyFolder: "暂无文件或文件夹",
  emptyFolderUploadHint: "拖放文件到这里，或点击上传文件。",
  uploadToFolder: "文件将添加到当前目录。",
  folderPath: "目录路径",
  archiveRoot: "根目录",
  boxRoot: "归档盒",
  returnToBoxRoot: "返回归档盒根目录",
  searchWholeBox: "搜索范围：当前目录及子目录",
  historyBoxTarget: "归档盒{target}",
  historyFileTarget: "文件{target}",
  historyNamed: "「{name}」",
  historyBatch: "「{name}」等 {count} 项",
  historyTagAdded: "为{target}添加标签「{tag}」",
  historyTagRemoved: "从{target}移除标签「{tag}」",
  historyRenamed: "「{before}」→「{after}」",
  historyCurrentNames: "按当前名称",
  historyCurrentNamesHint:
    "此操作只记录了对象 ID，引用的对象使用当前名称展示；名称不可用时保留 ID。",
  historySummaryUnavailable: "摘要暂不可用",
  historyAcceptedAt: "NAS 接纳时间",
  history: "操作历史",
  trash: "回收站",
  historyEmpty: "暂无操作记录。",
  trashHint: "已删除内容会保留，可恢复。",
  trashAllScopes: "所有记录",
  trashEmpty: "此范围内暂无已删除内容",
  restoreConfirm: "恢复本次删除的所有内容？若与后续更改冲突，将无法恢复。",
  historyScope: "历史范围",
  allHistory: "全部操作",
  operation: "操作 #{id}",
  undo: "撤销",
  restore: "恢复",
  undoConfirm:
    "将追加一条反向操作，作用于本次操作的所有条目。如与后续更改冲突，将拒绝执行。",
  undoneBy: "已由 #{id} 撤销",
  reverts: "撤销了 #{id}",
  blockedBy: "被操作 #{id} 阻挡",
  viewOperation: "查看操作",
  viewHistoricalBox: "查看此时的归档箱",
  historicalState: "操作 #{id} 之后的只读历史状态",
  versionHistory: "文件版本",
  compareVersions: "比较版本",
  beforeVersion: "较早操作编号",
  afterVersion: "较晚操作编号",
  compare: "对比",
  field: "字段",
  notPresent: "此时尚不存在或已删除",
  noDifferences: "两个时间点的状态没有差异。",
  changedFields: "变化的字段已高亮。",
  viewBeforeDeletion: "查看删除前的状态",
  affectedFiles: "涉及文件",
  affectedBoxes: "涉及归档箱",
  operationData: "操作记录详情",
  blobKey: "内容指纹",
  updated: "修改时间",
  origin: "来源",
  action_create_box: "创建归档箱",
  action_delete_box: "删除归档箱",
  action_delete_box_contents: "归档盒移入回收站",
  action_upload_files: "上传并覆盖文件",
  action_undo_upload_files: "撤销文件上传",
  uploadConflictTitle: "文件已存在",
  uploadConflictHint:
    "当前目录中已有以下同名条目。此选择适用于本批全部冲突：覆盖会保留旧内容的历史记录；保留两者会为新文件名添加序号。",
  uploadFolderConflict: "同名条目中包含文件夹，请选择保留两者。",
  uploadOverwrite: "覆盖",
  uploadKeepBoth: "保留两者",
  action_rename_entry: "重命名文件或目录",
  action_delete_entry_contents: "文件或目录移入回收站",
  action_restore_entry_contents: "恢复文件或目录",
  trashEntry: "移入回收站",
  trashEntryConfirm: "确定将「{name}」移入回收站吗？",
  trashFolderContents: "包含 {files} 个文件和 {folders} 个子目录。",
  trashEntryRestoreHint: "之后可以从回收站恢复。",
  action_restore_box_contents: "恢复归档盒",
  moveBoxToTrash: "移入回收站",
  trashBoxConfirm:
    "将“{name}”及其中全部 {count, number} 个文件移入回收站？目录、标签和元数据可一并恢复，原始文件会保留。",
  trashBoxChanged: "归档盒可能已发生变化，请关闭窗口并重试，重新确认当前内容。",
  action_update_box: "修改归档箱",
  savedSearches: "收藏夹",
  saveSearch: "收藏当前搜索",
  editSavedSearch: "编辑收藏搜索",
  replaceSavedSearch: "更新为当前搜索条件",
  replaceSearchHint: "保存后，将用当前搜索的范围、条件和排序替换原收藏。",
  removeSavedSearch: "取消收藏搜索",
  removeSavedSearchHint:
    "确认取消此收藏？文件和归档盒不受影响，可在操作历史中撤销。",
  savedSearchLiveHint: "结果随资料库内容实时更新",
  savedSearchMissing:
    "此搜索引用的标签、归档盒或目录已不存在，或条件版本不受支持。原条件已保留，请恢复引用对象或更新收藏条件。",
  action_create_saved_search: "收藏搜索",
  action_update_saved_search: "更新收藏搜索",
  action_delete_saved_search: "取消收藏搜索",
  action_create_tag: "创建标签",
  action_delete_tag: "删除标签",
  action_rename_tag: "重命名标签",
  action_move_tag: "移动标签",
  action_tag_boxes: "为归档箱添加标签",
  action_untag_boxes: "移除归档箱标签",
  action_create_folder: "新建目录",
  action_delete_folder: "删除目录",
  action_import_files: "导入文件",
  action_delete_files: "删除文件",
  action_extract_metadata: "提取元数据",
  action_set_metadata: "修改元数据",
  action_tag_files: "为文件添加标签",
  action_untag_files: "移除文件标签",
  action_request_receipt: "无变更回执",

  mediaNotCached: "此预览尚未缓存，请连接 NAS 后重试。",
  mediaCacheQuota: "缓存空间不足，请调整离线保留内容或缓存上限后重试。",
  mediaCacheMetadata: "预览缓存索引需要重建，请前往缓存设置处理。",
  mediaCacheCorrupt: "预览缓存校验失败，请重建缓存索引后重新下载。",
  mediaSyncVersion: "NAS 尚不支持兼容的预览传输，请升级后重试。",
  mediaQueued: "等待生成预览…",
  mediaRunning: "正在生成预览…",
  mediaDependency:
    "缺少预览工具：视频需要 FFmpeg，HEIC 需要带 HEVC 解码器的 libheif，PDF 需要 Poppler，Word/PowerPoint 需要 LibreOffice Writer/Impress（缩略图还需 Poppler）。安装后会自动重试。",
  mediaFailed: "预览生成失败，仍可下载原文件。",
  mediaCancelled: "已取消预览生成。",
  mediaLimit: "文件超出预览的尺寸或时长限制，请下载原文件查看。",
  mediaTimeout: "预览生成超时，可重试或下载原文件。",
  mediaRequestFailed: "无法读取或更新预览状态，请重试。",
  files: "文件",
  saving: "正在保存…",
  newOperation: "开始新操作",
  invalidInput: "请检查输入，并以修正后的内容开始新操作。",
  conflict: "归档状态已变化，或操作与现有数据冲突。请刷新后重试。",
  tagCycle: "不能将标签移动到自身的子树中。",
  deleteTagConfirm: "确认删除此标签？",
  deleteTagTitle: "删除标签",
  openTagPage: "打开标签页",
  viewTaggedBoxes: "查看归档盒",
  viewTaggedFiles: "查看文件",
  deleteNamedTagConfirm: "确定删除标签「{name}」吗？",
  uploadTooLarge: "这一批文件超过上传限制，请减少文件数量或大小。",
  appName: "kist",
  archive: "个人归档",
  boxes: "归档盒",
  allFiles: "全部文件",
  tags: "标签",
  allTags: "标签目录",
  settings: "显示设置",
  primaryColor: "主色",
  gray: "灰色",
  green: "绿色",
  yellow: "黄色",
  pink: "粉色",
  blue: "蓝色",
  orange: "橙色",
  colorMode: "明暗模式",
  modeSystem: "跟随系统",
  modeLight: "浅色",
  modeDark: "深色",
  language: "语言",
  formatLocale: "地区格式",
  timezone: "时区",
  direction: "阅读方向",
  auto: "自动",
  ltr: "从左到右",
  rtl: "从右到左",
  close: "关闭",
  save: "保存",
  cancel: "取消",
  loading: "正在加载…",
  retry: "重试",
  error: "操作未完成，请重试。",
  errorDetails: "操作未完成。{message}（错误码：{code}）",
  createBox: "新建归档盒",
  emptyBoxes: "留存值得珍藏的资料。",
  emptyBoxesHint: "为一次旅行、一段经历或一批档案创建归档盒。",
  name: "名称",
  type: "类型",
  description: "描述",
  startAt: "开始日期",
  endAt: "结束日期",
  today: "今天",
  useDateYear: "使用 {year} 年",
  useDateMonth: "使用 {year} 年 {month} 月",
  nextMonth: "下个月",
  previousMonth: "上个月",
  nextYear: "下一年",
  previousYear: "上一年",
  nextDecade: "下十年",
  previousDecade: "上十年",
  chooseMonth: "选择月份",
  chooseYear: "选择年份",
  dateHint: "填写年、年月或日期，例如 2026 或 2026-10-01",
  editBox: "编辑归档盒",
  upload: "添加文件",
  pasteUploadTitle: "上传粘贴的文件？",
  pasteUploadHint: "以下文件来自剪贴板，确认后才会上传。",
  search: "搜索文件名",
  noFiles: "这里还没有文件",
  noResults: "没有匹配的文件",
  importHint: "将文件拖到这里，或选择文件加入这个归档盒。",
  chooseFiles: "选择文件",
  uploadFiles: "上传文件",
  openItem: "打开",
  download: "下载原文件",
  openPDF: "在新标签页打开 PDF",
  openImage: "在新标签页打开图片",
  sheetView: "表格视图",
  sheetTable: "表格",
  sheetFailed: "暂时无法读取这份表格。CSV 可切换源码查看，也可以下载原文件。",
  sheetLimit: "这份表格的大小或展开后的数据量超过预览上限，请下载原文件查看。",
  sheetMissingFormula: "无已保存结果",
  sheetPreview: "表格预览",
  sheetReader: "表格阅读器",
  sheetWorksheet: "工作表",
  sheetSearch: "搜索当前页",
  sheetCopied: "已复制选区",
  sheetCopyFailed: "复制失败，请在表格中尝试 Ctrl/Cmd+C。",
  sheetCopy: "复制选区",
  sheetCachedValues: "显示已保存的公式结果，不重新计算",
  sheetLegacy: "旧版 XLS：部分格式可能不同",
  sheetTruncated: "过长的单元格内容已截断",
  sheetMatches: "个当前页匹配",
  sheetCells: "表格单元格",
  sheetEmpty: "当前页没有单元格",
  sheetPreviousRows: "上一组行",
  sheetNextRows: "下一组行",
  sheetPreviousColumns: "上一组列",
  sheetNextColumns: "下一组列",
  markdownMode: "Markdown 视图",
  markdownPreview: "预览",
  markdownSource: "源码",
  markdownMissing: "引用的文件暂时无法访问",
  markdownExternalImage: "外部图片",
  markdownImageBlocked: "未自动加载",
  markdownOpenImage: "打开图片",
  textPreview: "文本预览",
  textReader: "文本阅读器",
  textEncoding: "文本编码",
  textEncodingAuto: "自动识别编码",
  textLoadFailed: "文本加载失败。",
  textEncodingUnknown:
    "暂时无法解码这份文本。已支持 UTF-8、UTF-16 和 GBK/GB18030；文件可能使用了其他编码，或含有无效字节。你仍可下载原文件查看。",
  emptyText: "空文本文件",
  plainText: "纯文本",
  textWrap: "自动换行",
  textSearch: "搜索文本",
  textNoMatches: "已加载文本中没有匹配结果。",
  textTruncated:
    "当前仅显示文件开头（最多 1 MiB），搜索仅覆盖已加载内容。完整内容请下载原文件。",
  imageLoadFailed: "原图加载失败。",
  pdfZoomIn: "放大",
  pdfZoomOut: "缩小",
  pdfTools: "PDF 工具栏",
  pdfPage: "页码",
  pdfPages: "PDF 页面",
  pdfPrevious: "上一页",
  pdfNext: "下一页",
  pdfFitWidth: "适合宽度",
  pdfSearch: "搜索 PDF 文本",
  pdfPreviousMatch: "上一个匹配",
  pdfNextMatch: "下一个匹配",
  pdfUnavailable:
    "无法读取此 PDF，文件可能已损坏或需要密码。请在新标签页打开或下载原文件。",
  details: "文件详情",
  basicInfo: "基本信息",
  mediaKind: "类型",
  mimeType: "MIME 类型",
  openNewTab: "在新标签页打开",
  copyLink: "复制链接",
  copyLinkFailed: "无法复制链接",
  uploadHere: "上传到此目录",
  copyFileId: "复制文件 ID",
  copy: "复制",
  copied: "已复制",
  imageHeight: "图像高度",
  imageWidth: "图像宽度",
  resolutionUnit: "分辨率单位",
  horizontalResolution: "水平分辨率",
  verticalResolution: "垂直分辨率",
  userComment: "备注",
  cameraMake: "相机厂商",
  cameraModel: "相机型号",
  software: "软件",
  previewUnavailable: "暂不支持预览此格式，仍可下载原文件。",
  image: "图片",
  video: "视频",
  audio: "音频",
  document: "文档",
  other: "其他",
  unknown: "尚未分类",
  kind: "文件类型",
  allTypes: "所有类型",
  sort: "排序",
  captured: "内容时间",
  imported: "导入顺序",
  filename: "文件名",
  ascending: "升序",
  descending: "降序",
  addFilter: "添加条件",
  searchConditions: "搜索或添加条件…",
  contains: "包含",
  operator: "运算符",
  chooseOperator: "选择运算符…",
  executeSearch: "搜索",
  chooseFilter: "选择条件",
  filterValue: "输入以选择条件值…",
  searchText: "搜索此文本",
  applyFilter: "应用",
  filterDateHint: "内容日期：年、年-月或年-月-日",
  noFilterOptions: "没有匹配的选项",
  from: "开始日期",
  to: "结束日期",
  clearFilters: "清除筛选",
  adjustSearchHint: "试试调整或清除搜索条件",
  loadMore: "加载更多",
  searchBoxes: "搜索归档盒名称和描述",
  boxDateHint: "筛选与活动时间重叠的范围，支持 YYYY、YYYY-MM 或 YYYY-MM-DD。",
  activityDate: "活动日期",
  sameYear: "当年",
  sameMonth: "当月",
  sameDay: "当天",
  sameMoment: "同一时刻",
  allBoxesShown: "已全部显示 {count, number} 个归档盒",
  gallery: "画廊",
  galleryNavigation: "浏览文件",
  backToResults: "返回列表",
  previousFile: "上一个文件",
  nextFile: "下一个文件",
  singleFile: "单个文件",
  displayMode: "显示方式",
  largeIcons: "大图标",
  mediumIcons: "中图标",
  smallIcons: "小图标",
  listView: "列表",
  fileSize: "大小",
  allFoldersShown: "已全部显示 {count, number} 个文件夹",
  allEntriesShown:
    "已全部显示 {folders, number} 个文件夹、{files, number} 个文件",
  allFilesShown: "已全部显示 {count, number} 个文件",
  filters: "筛选",
  tagAll: "同时包含这些标签",
  tagAny: "包含任一标签",
  tagNot: "排除这些标签",
  newTag: "新建标签",
  rename: "重命名",
  tagActions: "{name}的操作",
  addChildTag: "添加子标签",
  changeParentTag: "更改父标签",
  move: "移动",
  remove: "删除",
  parentTag: "父标签",
  rootTag: "无父标签",
  manageTags: "管理标签",
  addTag: "添加标签",
  removeTag: "移除标签",
  selectTag: "选择标签",
  searchTags: "搜索标签",
  searchOptions: "搜索选项",
  tagResultCounts: "{boxes, number} 个归档盒 · {files, number} 个文件",
  noMatchingBoxes: "没有匹配的归档盒",
  noTaggedFilesHint: "给文件添加此标签或子标签后，就会显示在这里。",
  expandTag: "展开{name}",
  collapseTag: "折叠{name}",
  created: "添加时间",
  modified: "更新时间",
  capturedAt: "内容时间",
  metadata: "元数据",
  path: "路径",
  fileId: "文件 ID",
  extractionPending: "尚未完整读取元数据。",
  extract: "读取元数据",
  extractionFailed: "部分元数据读取失败，原文件已保存。",
  extracting: "正在读取元数据…",
  uploadCancelled: "上传已取消。开始新上传前，请核对提交结果。",
  uploadFailed: "上传中断。使用相同文件重试，可安全恢复提交结果。",
  uploadComplete: "文件已保存",
  uploadElapsed: "上传完成 · 本次耗时 {seconds} 秒 · 平均 {speed}/秒",
  uploadCurrentStats: "已用时 {seconds} 秒 · 平均有效传输 {speed}/秒",
  uploadTiming:
    "读取与校验 {hash} 秒；分块请求（含服务端写盘）{transfer} 秒；最终处理 {processing} 秒。读取与传输有重叠，各阶段不能直接相加；不含暂停时间。",
  uploading: "正在上传…",
  uploadHashing: "正在计算文件校验值…",
  uploadPause: "暂停",
  uploadResume: "继续上传",
  uploadReselect: "重新选择原文件",
  uploadChecking: "正在核对已保存的上传进度…",
  uploadPaused: "已暂停，服务端已确认 {percent}%。",
  uploadReselectHint:
    "刷新或重新打开网页后，需要重新选择文件，网页才能再次读取。请选择上次上传的全部原文件；核对一致后会自动续传，已确认的部分不会重复上传。",
  uploadWrongSource:
    "所选文件与原批次不一致。请重新选择原文件；文件名、大小和修改时间相同并不足以续传。",
  uploadStorageUnavailable:
    "无法保存或读取续传信息。请检查此网站的浏览器存储权限与空间，然后刷新页面。",
  uploadProtocolError:
    "服务端返回的续传进度不兼容。请刷新并核对结果，再开始新的上传。",
  uploadResumeFailed: "上传中断，已确认的数据仍保留。连接恢复后可以继续上传。",
  uploadExpired: "上传会话已过期或被移除。请取消此记录，再选择文件开始新上传。",
  uploadTargetChanged:
    "目标归档盒、目录或同名文件已发生变化。请取消本次上传，重新选择文件并确认冲突处理方式。",
  uploadStorageFull: "服务端磁盘空间不足。释放空间后可继续本次上传。",
  uploadSessionQuota:
    "服务端暂存上传已达到额度。请先完成或取消其他上传，再重试。",
  uploadSessionDamaged:
    "暂存会话损坏或版本不兼容。请核对结果与服务端日志后再清理。",
  uploadBackgroundHint:
    "无需一直盯着页面，请保持页面打开、设备不休眠。若浏览器挂起，可从已确认的位置继续；数据保留至空闲 7 天。",
  uploadProcessing: "服务端正在校验并处理文件…",
  uploadIntegrityMismatch:
    "文件校验不一致，本批次未归档。请重新选择原文件后重试。",
  checkResult: "核对结果",
  requestPending: "尚无已提交结果，可以重试此请求。",
  projectionDirty:
    "人可读目录需要修复。归档数据已保存，可运行 kist relink 重建目录。",
  loadedFiles: "已加载 {count, number} 个文件",
  fileCount: "{count, number} 个文件",
  uploadLimit: "每批最多 {count} 个文件、{size}。",
  uploadProgress: "已上传 {percent}%",
  fieldRequired: "请输入名称。",
  invalidDate: "请输入有效的年、年月或日期。",
  invalidSettings: "请输入有效的地区格式和时区。",
  requestConflict: "此请求标识已用于不同内容，请开始新操作。",
  notFound: "此项已不存在，请刷新归档。",
  refresh: "刷新",
  addItems: "添加内容",
  moreActions: "更多操作",
};

export interface Preferences {
  language: "en" | "zh";
  formatLocale: string;
  timezone: string;
  direction: "auto" | "ltr" | "rtl";
}
export const storageKey = "kist.display.v1";
export function validFormats(locale: string, timezone: string): boolean {
  try {
    new Intl.NumberFormat(locale);
    new Intl.DateTimeFormat(locale, { timeZone: timezone });
    return true;
  } catch {
    return false;
  }
}
export function initialPreferences(): Preferences {
  const locale = navigator.languages?.[0] || "en-US";
  const defaults: Preferences = {
    language: locale.startsWith("zh") ? "zh" : "en",
    formatLocale: locale,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    direction: "auto",
  };
  try {
    const saved = JSON.parse(window.localStorage.getItem(storageKey) || "null");
    if (
      saved &&
      ["en", "zh"].includes(saved.language) &&
      ["auto", "ltr", "rtl"].includes(saved.direction) &&
      typeof saved.formatLocale === "string" &&
      typeof saved.timezone === "string" &&
      validFormats(saved.formatLocale, saved.timezone)
    )
      return saved;
  } catch {
    /* Storage may be unavailable or contain an older format. */
  }
  return defaults;
}
export function directionFor(prefs: Preferences): "ltr" | "rtl" {
  if (prefs.direction !== "auto") return prefs.direction;
  // Message languages currently shipped are both LTR. Direction remains
  // independent of regional number and date formatting.
  return "ltr";
}
function applyDocumentLanguage(prefs: Preferences) {
  document.documentElement.lang = prefs.language;
  document.documentElement.dir = directionFor(prefs);
}
export function initializeLanguage() {
  applyDocumentLanguage(initialPreferences());
}
interface Parameters {
  clientSelectLoaded: { count: number };
  clientSelectedCount: { count: number };
  clientBatchProgress: { retained: number; prepared: number; total: number };
  clientRetainedCount: { files: number; filesets: number };
  clientOfflineMissing: { ready: number; total: number };
  clientOfflinePreviews: { ready: number; total: number };
  clientPending: { count: number };
  clientCacheUsed: { size: string };
  clientPinnedUsed: { size: string };
  clientProtectedUsed: { originals: string; journal: string };
  historyBoxTarget: { target: string };
  historyFileTarget: { target: string };
  historyNamed: { name: string };
  historyBatch: { name: string; count: number };
  historyTagAdded: { target: string; tag: string };
  historyTagRemoved: { target: string; tag: string };
  historyRenamed: { before: string; after: string };
  trashEntryConfirm: { name: string };
  trashFolderContents: { files: number; folders: number };
  deleteNamedTagConfirm: { name: string };
  uploadElapsed: { seconds: string; speed: string };
  uploadCurrentStats: { seconds: string; speed: string };
  uploadTiming: { hash: string; transfer: string; processing: string };
  tagResultCounts: { boxes: number; files: number };
  trashBoxConfirm: { name: string; count: number };
  tagActions: { name: string };
  expandTag: { name: string };
  filesetCurrentPage: { page: number };
  filesetDragPage: { name: string };
  filesetRemovePage: { name: string };
  filesetReordered: { page: number };
  collapseTag: { name: string };
  operation: { id: number };
  clientFormalOperation: { id: number };
  undoneBy: { id: number };
  reverts: { id: number };
  blockedBy: { id: number };
  historicalState: { id: number };
  errorDetails: { message: string; code: string };
  loadedFiles: { count: number };
  allFilesShown: { count: number };
  allFoldersShown: { count: number };
  allEntriesShown: { folders: number; files: number };
  allBoxesShown: { count: number };
  fileCount: { count: number };
  useDateYear: { year: string };
  useDateMonth: { year: string; month: string };
  uploadLimit: { count: string; size: string };
  uploadProgress: { percent: string };
  uploadPaused: { percent: string };
}
export type Translate = <K extends MessageKey>(
  key: K,
  ...args: K extends keyof Parameters ? [Parameters[K]] : []
) => string;
import { IntlMessageFormat } from "intl-messageformat";
export function translator(prefs: Preferences): Translate {
  const catalog = prefs.language === "zh" ? zh : en;
  return ((key: MessageKey, values?: Record<string, string | number>) =>
    new IntlMessageFormat(catalog[key] || en[key], prefs.language, undefined, {
      formatters: {
        getNumberFormat: (_, options) =>
          new Intl.NumberFormat(prefs.formatLocale, options),
        getDateTimeFormat: (_, options) =>
          new Intl.DateTimeFormat(prefs.formatLocale, {
            ...options,
            timeZone: prefs.timezone,
          }),
        getPluralRules: (_, options) =>
          new Intl.PluralRules(prefs.language, options),
      },
    }).format(values) as string) as Translate;
}
const I18nContext = createContext<{
  prefs: Preferences;
  setPrefs: (p: Preferences) => void;
  t: Translate;
} | null>(null);
export function I18nProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState(initialPreferences);
  const { setDirection } = useDirection();
  useLayoutEffect(() => {
    applyDocumentLanguage(prefs);
    setDirection(directionFor(prefs));
    try {
      window.localStorage.setItem(storageKey, JSON.stringify(prefs));
    } catch {
      /* The current session still retains preferences. */
    }
  }, [prefs, setDirection]);
  return (
    <I18nContext value={{ prefs, setPrefs, t: translator(prefs) }}>
      {children}
    </I18nContext>
  );
}
export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) throw new Error("I18nProvider is missing");
  return value;
}
export function formatNumber(value: number, prefs: Preferences): string {
  return new Intl.NumberFormat(prefs.formatLocale).format(value);
}
export function formatBytes(value: number, prefs: Preferences): string {
  const units = ["byte", "kilobyte", "megabyte", "gigabyte", "terabyte"];
  const power = Math.max(
    0,
    Math.min(units.length - 1, Math.floor(Math.log10(Math.max(1, value)) / 3)),
  );
  return new Intl.NumberFormat(prefs.formatLocale, {
    style: "unit",
    unit: units[power],
    unitDisplay: "short",
    maximumFractionDigits: 1,
  }).format(value / 1000 ** power);
}
export function formatInstant(value: string, prefs: Preferences): string {
  return new Intl.DateTimeFormat(prefs.formatLocale, {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: prefs.timezone,
  }).format(new Date(value));
}
// LocalDate is a civil date with significant precision, not an instant. Use a
// fixed Gregorian calendar and UTC only as an Intl formatting carrier.
export function formatLocalDate(
  value: string,
  prefs: Preferences,
  rangeStart?: string,
): string {
  if (!/^\d{4}(-\d{2}(-\d{2}(T\d{2}:\d{2})?)?)?$/.test(value)) return value;
  const [year, month = 1, day = 1, hour = 0, minute = 0] = value
    .split(/[-T:]/)
    .map(Number);
  const carrier = new Date(0);
  carrier.setUTCFullYear(year, month - 1, day);
  carrier.setUTCHours(hour, minute, 0, 0);
  return new Intl.DateTimeFormat(prefs.formatLocale, {
    calendar: "gregory",
    timeZone: "UTC",
    ...(rangeStart?.length === 10 &&
    value.length === 10 &&
    rangeStart.slice(0, 4) === value.slice(0, 4)
      ? {}
      : { year: "numeric" as const }),
    ...(value.length >= 7 &&
      !(
        rangeStart?.length === 10 &&
        value.length === 10 &&
        rangeStart.slice(0, 7) === value.slice(0, 7)
      ) && { month: "long" as const }),
    ...(value.length >= 10 && { day: "numeric" }),
    ...(value.length >= 16 && { hour: "numeric", minute: "2-digit" }),
  }).format(carrier);
}
