import { fetchJSON, fetchURL, removePrefix } from "./utils";

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

export async function getMediaInfo(url: string): Promise<MediaInfoResponse> {
  const path = removePrefix(url);
  return fetchJSON<MediaInfoResponse>(`/api/mediainfo${path}`, {});
}

export async function startBDInfoScan(
  url: string,
  force = false
): Promise<BDInfoTask> {
  const path = removePrefix(url);
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
