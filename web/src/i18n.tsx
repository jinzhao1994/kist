import { useDirection } from "@mantine/core";
import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

export const en = {
  searchTags: "Search tags",
  tagResultCounts:
    "{boxes, plural, one {# box} other {# boxes}} · {files, plural, one {# file} other {# files}}",
  noMatchingBoxes: "No matching boxes",
  noTaggedFilesHint:
    "Files appear here when assigned this tag or one of its subtags.",
  expandTag: "Expand {name}",
  collapseTag: "Collapse {name}",
  folders: "Folders",
  createFolder: "New folder",
  emptyFolder: "This folder is empty",
  uploadToFolder: "Files will be added to the current folder.",
  folderPath: "Folder path",
  boxRoot: "Box root",
  searchWholeBox: "Searching the entire box",
  history: "History",
  trash: "Recycle bin",
  historyEmpty: "No operations here yet.",
  trashHint:
    "Deleted items are retained. Restore an entire operation; later changes may prevent restoration.",
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
  action_restore_box_contents: "Restore box",
  moveBoxToTrash: "Move to trash",
  trashBoxConfirm:
    "Move “{name}” and all {count, number} files to trash? Folders, tags and metadata will be restored together. Original files are retained.",
  trashBoxChanged:
    "The box may have changed. Close this dialog and try again to review its current contents.",
  action_update_box: "Edit box",
  savedSearches: "Saved searches",
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

  mediaQueued: "Waiting to generate preview…",
  mediaRunning: "Generating preview…",
  mediaDependency:
    "Preview tools are missing. Video needs FFmpeg; HEIC needs libheif with an HEVC decoder. Will retry automatically.",
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
  uploadTooLarge:
    "The batch exceeds the configured upload limits. Choose fewer or smaller files.",
  appName: "kist",
  archive: "Your archive",
  boxes: "Boxes",
  allFiles: "All files",
  tags: "Tags",
  allTags: "All tags",
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
  description: "Description",
  startAt: "Start date",
  endAt: "End date",
  dateHint: "Year, month or date (for example, 2026 or 2026-10-01)",
  editBox: "Edit box",
  upload: "Add files",
  search: "Search filenames",
  noFiles: "No files here yet",
  noResults: "No matching files",
  importHint: "Drop files here, or choose files to add to this box.",
  chooseFiles: "Choose files",
  download: "Download original",
  details: "File details",
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
  captured: "Capture time",
  imported: "Import order",
  filename: "Filename",
  ascending: "Ascending",
  descending: "Descending",
  addFilter: "Add filter",
  searchConditions: "Search or add a condition…",
  chooseFilter: "Choose a condition",
  filterValue: "Type to choose a value…",
  searchText: "Search names for “{value}”",
  applyFilter: "Apply",
  filterDateHint: "Capture date: YYYY, YYYY-MM or YYYY-MM-DD",
  noFilterOptions: "No matching options",
  from: "From",
  to: "Until",
  clearFilters: "Clear filters",
  loadMore: "Load more",
  searchBoxes: "Search box names and descriptions",
  boxDateHint:
    "Activity interval overlaps this range. Use YYYY, YYYY-MM or YYYY-MM-DD.",
  activityDate: "Activity date",
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
  capturedAt: "Captured",
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
  uploading: "Uploading…",
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
  moreActions: "More actions",
} as const;
export type MessageKey = keyof typeof en;
export const zh: Record<MessageKey, string> = {
  folders: "文件夹",
  createFolder: "新建目录",
  emptyFolder: "此目录为空",
  uploadToFolder: "文件将添加到当前目录。",
  folderPath: "目录路径",
  boxRoot: "根目录",
  searchWholeBox: "正在搜索整个归档盒",
  history: "操作历史",
  trash: "回收站",
  historyEmpty: "暂无操作记录。",
  trashHint:
    "已删除内容会永久保留。恢复以整条操作为单位，后续更改可能阻止恢复。",
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
  action_restore_box_contents: "恢复归档盒",
  moveBoxToTrash: "移入回收站",
  trashBoxConfirm:
    "将“{name}”及其中全部 {count, number} 个文件移入回收站？目录、标签和元数据可一并恢复，原始文件会保留。",
  trashBoxChanged: "归档盒可能已发生变化，请关闭窗口并重试，重新确认当前内容。",
  action_update_box: "修改归档箱",
  savedSearches: "收藏的搜索",
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

  mediaQueued: "等待生成预览…",
  mediaRunning: "正在生成预览…",
  mediaDependency:
    "缺少预览工具：视频需要 FFmpeg，HEIC 需要带 HEVC 解码器的 libheif。安装后会自动重试。",
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
  uploadTooLarge: "这一批文件超过上传限制，请减少文件数量或大小。",
  appName: "kist",
  archive: "个人归档",
  boxes: "归档盒",
  allFiles: "全部文件",
  tags: "标签",
  allTags: "所有标签",
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
  description: "描述",
  startAt: "开始日期",
  endAt: "结束日期",
  dateHint: "填写年、年月或日期，例如 2026 或 2026-10-01",
  editBox: "编辑归档盒",
  upload: "添加文件",
  search: "搜索文件名",
  noFiles: "这里还没有文件",
  noResults: "没有匹配的文件",
  importHint: "将文件拖到这里，或选择文件加入这个归档盒。",
  chooseFiles: "选择文件",
  download: "下载原文件",
  details: "文件详情",
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
  captured: "拍摄时间",
  imported: "导入顺序",
  filename: "文件名",
  ascending: "升序",
  descending: "降序",
  addFilter: "添加条件",
  searchConditions: "搜索或添加条件…",
  chooseFilter: "选择条件",
  filterValue: "输入以选择条件值…",
  searchText: "搜索名称“{value}”",
  applyFilter: "应用",
  filterDateHint: "拍摄日期：年、年-月或年-月-日",
  noFilterOptions: "没有匹配的选项",
  from: "从",
  to: "至",
  clearFilters: "清除筛选",
  loadMore: "加载更多",
  searchBoxes: "搜索归档盒名称和描述",
  boxDateHint: "筛选与活动时间重叠的范围，支持 YYYY、YYYY-MM 或 YYYY-MM-DD。",
  activityDate: "活动日期",
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
  allFilesShown: "已全部显示 {count, number} 个文件",
  filters: "筛选",
  tagAll: "同时包含这些标签",
  tagAny: "包含任一标签",
  tagNot: "排除这些标签",
  newTag: "新建标签",
  rename: "改名",
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
  tagResultCounts: "{boxes, number} 个归档盒 · {files, number} 个文件",
  noMatchingBoxes: "没有匹配的归档盒",
  noTaggedFilesHint: "给文件添加此标签或子标签后，就会显示在这里。",
  expandTag: "展开{name}",
  collapseTag: "折叠{name}",
  created: "添加时间",
  modified: "更新时间",
  capturedAt: "拍摄时间",
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
  uploading: "正在上传…",
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
interface Parameters {
  searchText: { value: string };
  tagResultCounts: { boxes: number; files: number };
  trashBoxConfirm: { name: string; count: number };
  tagActions: { name: string };
  expandTag: { name: string };
  collapseTag: { name: string };
  operation: { id: number };
  undoneBy: { id: number };
  reverts: { id: number };
  blockedBy: { id: number };
  historicalState: { id: number };
  errorDetails: { message: string; code: string };
  loadedFiles: { count: number };
  allFilesShown: { count: number };
  allBoxesShown: { count: number };
  fileCount: { count: number };
  uploadLimit: { count: string; size: string };
  uploadProgress: { percent: string };
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
  useEffect(() => {
    document.documentElement.lang = prefs.language;
    document.documentElement.dir = directionFor(prefs);
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
export function formatLocalDate(value: string, prefs: Preferences): string {
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
    year: "numeric",
    ...(value.length >= 7 && { month: "long" }),
    ...(value.length >= 10 && { day: "numeric" }),
    ...(value.length >= 16 && { hour: "numeric", minute: "2-digit" }),
  }).format(carrier);
}
