import { fetchJSON, fetchURL, removePrefix } from "./utils";
import type { ImageHostPreset } from "@/utils/prefs";

export interface MediaInfoSummary {
  format?: string;
  fileSize?: string;
  duration?: string;
  bitRate?: string;
  video?: string[];
  audio?: string[];
  subtitles: number;
  hasHdr: boolean;
  hdrLabel?: string;
}

export interface MediaInfoResponse {
  path: string;
  name: string;
  text: string;
  summary: MediaInfoSummary;
}

export interface BDInfoSummary {
  discTitle?: string;
  discSize?: string;
  playlist?: string;
  length?: string;
  bitRate?: string;
  video?: string;
  audioCount: number;
  subtitleCount: number;
}

export interface BDInfoTask {
  id: string;
  path: string;
  targetPath: string;
  status: "running" | "done" | "error" | "canceled";
  progress: number;
  stage: string;
  summary?: BDInfoSummary;
  quickSummary?: string;
  fullReport?: string;
  error?: string;
  startedAt: string;
  finishedAt?: string;
  elapsedSec: number;
}

export interface FastDLSignResponse {
  mode: "file" | "vtar" | "manifest";
  filename: string;
  url: string;
  expiresAt: string;
  fileCount: number;
  totalSize: number;
  aria2Cmd: string;
  curlCmd: string;
  manifestUrl?: string;
  aria2ManifestCmd?: string;
}

export interface ScreenshotCaptureParams {
  count: number;
  format: "png" | "jpg" | "webp";
  quality: number;
  startPct: number;
  endPct: number;
  timestamps?: string[];
  hdrToneMap: "auto" | "on" | "off";
  width: number;
}

export interface ScreenshotItem {
  id: string;
  index: number;
  timestamp: string;
  filename: string;
  mimeType: string;
  size: number;
  width: number;
  height: number;
  previewUrl: string;
}

export interface ScreenshotCaptureResponse {
  path: string;
  source: string;
  duration: number;
  isHdr: boolean;
  toneMapApplied: boolean;
  items: ScreenshotItem[];
}

export interface UploadedShotResult {
  shotId: string;
  filename: string;
  url?: string;
  thumbUrl?: string;
  bbcode?: string;
  markdown?: string;
  error?: string;
}

export interface ScreenshotUploadResponse {
  results: UploadedShotResult[];
  bbcode: string;
  markdown: string;
  direct: string;
}

export async function getMediaInfo(url: string): Promise<MediaInfoResponse> {
  const path = removePrefix(url);
  return fetchJSON<MediaInfoResponse>(`/api/mediainfo${path}`, {});
}

export async function startBDInfoScan(
  url: string,
  force = false
): Promise<BDInfoTask> {
  const path = decodeURIComponent(removePrefix(url));
  return fetchJSON<BDInfoTask>("/api/bdinfo/scan", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path, force }),
  });
}

export async function getBDInfoTask(id: string): Promise<BDInfoTask> {
  return fetchJSON<BDInfoTask>(`/api/bdinfo/tasks/${encodeURIComponent(id)}`, {});
}

export async function cancelBDInfoTask(id: string): Promise<void> {
  await fetchURL(`/api/bdinfo/tasks/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export async function signFastDownload(
  urls: string[],
  mode: "" | "file" | "vtar" | "manifest" = "",
  ttlHours = 6
): Promise<FastDLSignResponse> {
  const paths = urls.map((u) => {
    const cleaned = removePrefix(u);
    return decodeURIComponent(cleaned);
  });
  return fetchJSON<FastDLSignResponse>("/api/fastdl/sign", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ paths, mode, ttlHours }),
  });
}

export async function captureScreenshots(
  url: string,
  params: ScreenshotCaptureParams
): Promise<ScreenshotCaptureResponse> {
  const path = decodeURIComponent(removePrefix(url));
  return fetchJSON<ScreenshotCaptureResponse>("/api/screenshots/capture", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path, ...params }),
  });
}

export async function uploadScreenshots(
  shotIds: string[],
  host: ImageHostPreset
): Promise<ScreenshotUploadResponse> {
  return fetchJSON<ScreenshotUploadResponse>("/api/screenshots/upload", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      shotIds,
      host: {
        provider: host.provider,
        endpoint: host.endpoint,
        apiKey: host.apiKey,
        fieldName: host.fieldName,
        authHeader: host.authHeader,
        authPrefix: host.authPrefix,
        urlJsonPath: host.urlJsonPath,
      },
    }),
  });
}
