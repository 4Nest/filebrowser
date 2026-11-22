export type ImageHostProvider =
  | "picx"
  | "pixhost"
  | "chevereto"
  | "lsky"
  | "custom";

export interface ImageHostPreset {
  id: string;
  name: string;
  provider: ImageHostProvider;
  endpoint: string;
  apiKey: string;
  fieldName: string;
  authHeader: string;
  authPrefix: string;
  urlJsonPath: string;
}

export interface LocalPrefs {
  fastdlTtlHours: number;
  aria2Threads: number;
  bdinfoDefaultTab: "quick" | "bbcode" | "full";
  screenshotCount: number;
  screenshotFormat: "png" | "jpg" | "webp";
  screenshotQuality: number;
  screenshotStartPct: number;
  screenshotEndPct: number;
  screenshotHdrToneMap: "auto" | "on" | "off";
  screenshotWidth: number;
  activeImageHostId: string;
  imageHosts: ImageHostPreset[];
}

const STORAGE_KEY = "fb_nest_prefs_v2";

export const DEFAULT_IMAGE_HOSTS: ImageHostPreset[] = [
  {
    id: "picx",
    name: "PICX (picx.cx)",
    provider: "picx",
    endpoint: "https://picx.cx/api/1/upload",
    apiKey: "",
    fieldName: "source",
    authHeader: "X-API-Key",
    authPrefix: "",
    urlJsonPath: "image.url",
  },
  {
    id: "pixhost",
    name: "Pixhost (免 Key)",
    provider: "pixhost",
    endpoint: "https://api.pixhost.to/images",
    apiKey: "",
    fieldName: "img",
    authHeader: "",
    authPrefix: "",
    urlJsonPath: "th_url",
  },
  {
    id: "chevereto",
    name: "通用 Chevereto",
    provider: "chevereto",
    endpoint: "https://your-host.com/api/1/upload",
    apiKey: "",
    fieldName: "source",
    authHeader: "X-API-Key",
    authPrefix: "",
    urlJsonPath: "image.url",
  },
  {
    id: "lsky",
    name: "兰空图床 (Lsky Pro)",
    provider: "lsky",
    endpoint: "https://your-lsky.com/api/v1/upload",
    apiKey: "",
    fieldName: "file",
    authHeader: "Authorization",
    authPrefix: "Bearer ",
    urlJsonPath: "data.links.url",
  },
  {
    id: "custom",
    name: "自定义图床",
    provider: "custom",
    endpoint: "",
    apiKey: "",
    fieldName: "file",
    authHeader: "Authorization",
    authPrefix: "Bearer ",
    urlJsonPath: "data.url",
  },
];

const DEFAULTS: LocalPrefs = {
  fastdlTtlHours: 6,
  aria2Threads: 16,
  bdinfoDefaultTab: "quick",
  screenshotCount: 6,
  screenshotFormat: "png",
  screenshotQuality: 92,
  screenshotStartPct: 10,
  screenshotEndPct: 90,
  screenshotHdrToneMap: "auto",
  screenshotWidth: 0,
  activeImageHostId: "picx",
  imageHosts: DEFAULT_IMAGE_HOSTS.map((h) => ({ ...h })),
};

export function getLocalPrefs(): LocalPrefs {
  try {
    const raw =
      localStorage.getItem(STORAGE_KEY) ||
      localStorage.getItem("fb_nest_prefs_v1");
    if (!raw) {
      return {
        ...DEFAULTS,
        imageHosts: DEFAULT_IMAGE_HOSTS.map((h) => ({ ...h })),
      };
    }
    const parsed = JSON.parse(raw);
    const mergedHosts: ImageHostPreset[] =
      Array.isArray(parsed.imageHosts) && parsed.imageHosts.length > 0
        ? parsed.imageHosts
        : DEFAULT_IMAGE_HOSTS.map((h) => ({ ...h }));

    return {
      fastdlTtlHours: [2, 6, 24, 48].includes(parsed.fastdlTtlHours)
        ? parsed.fastdlTtlHours
        : DEFAULTS.fastdlTtlHours,
      aria2Threads: [8, 16, 32].includes(parsed.aria2Threads)
        ? parsed.aria2Threads
        : DEFAULTS.aria2Threads,
      bdinfoDefaultTab: ["quick", "bbcode", "full"].includes(
        parsed.bdinfoDefaultTab
      )
        ? parsed.bdinfoDefaultTab
        : DEFAULTS.bdinfoDefaultTab,
      screenshotCount:
        typeof parsed.screenshotCount === "number" &&
        parsed.screenshotCount >= 1 &&
        parsed.screenshotCount <= 12
          ? parsed.screenshotCount
          : DEFAULTS.screenshotCount,
      screenshotFormat: ["png", "jpg", "webp"].includes(parsed.screenshotFormat)
        ? parsed.screenshotFormat
        : DEFAULTS.screenshotFormat,
      screenshotQuality:
        typeof parsed.screenshotQuality === "number" &&
        parsed.screenshotQuality >= 70 &&
        parsed.screenshotQuality <= 100
          ? parsed.screenshotQuality
          : DEFAULTS.screenshotQuality,
      screenshotStartPct:
        typeof parsed.screenshotStartPct === "number"
          ? parsed.screenshotStartPct
          : DEFAULTS.screenshotStartPct,
      screenshotEndPct:
        typeof parsed.screenshotEndPct === "number"
          ? parsed.screenshotEndPct
          : DEFAULTS.screenshotEndPct,
      screenshotHdrToneMap: ["auto", "on", "off"].includes(
        parsed.screenshotHdrToneMap
      )
        ? parsed.screenshotHdrToneMap
        : DEFAULTS.screenshotHdrToneMap,
      screenshotWidth:
        typeof parsed.screenshotWidth === "number"
          ? parsed.screenshotWidth
          : DEFAULTS.screenshotWidth,
      activeImageHostId:
        typeof parsed.activeImageHostId === "string" &&
        mergedHosts.some((h) => h.id === parsed.activeImageHostId)
          ? parsed.activeImageHostId
          : mergedHosts[0].id,
      imageHosts: mergedHosts,
    };
  } catch {
    return {
      ...DEFAULTS,
      imageHosts: DEFAULT_IMAGE_HOSTS.map((h) => ({ ...h })),
    };
  }
}

export function saveLocalPrefs(prefs: Partial<LocalPrefs>): LocalPrefs {
  const next = { ...getLocalPrefs(), ...prefs };
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  } catch {
    // ignore storage quota errors
  }
  return next;
}
