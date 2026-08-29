const TOKEN_KEY = "netodrive_token";

export type FileMeta = {
  id: number;
  path: string;
  name: string;
  is_dir: boolean;
  size: number;
  hash: string;
  mime: string;
  mtime: string;
  version: number;
  gallery_key?: string;
  width?: number;
  height?: number;
};

function apiBase() {
  return import.meta.env.VITE_API_URL?.replace(/\/$/, "") || "";
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || "";
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(`${apiBase()}${path}`, { ...init, headers });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const data = await res.json();
      if (data?.error) msg = data.error;
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function login(username: string, password: string) {
  return request<{ token: string; username: string }>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export function listFiles(path: string) {
  const q = new URLSearchParams({ path });
  return request<{ path: string; files: FileMeta[] }>(`/api/files?${q}`);
}

export function createDir(path: string) {
  return request<FileMeta>(`/api/files/${encodePath(path)}`, {
    method: "POST",
    body: JSON.stringify({ is_dir: true }),
  });
}

export function deleteFile(path: string) {
  return request<FileMeta>(`/api/files/${encodePath(path)}`, { method: "DELETE" });
}

export async function uploadFile(path: string, file: File) {
  const headers: Record<string, string> = {
    Authorization: `Bearer ${getToken()}`,
    "X-File-Path": path,
    "X-File-Mime": file.type || "application/octet-stream",
    "X-File-Mtime": new Date(file.lastModified).toISOString(),
    "X-Device-Id": "web",
  };
  const res = await fetch(`${apiBase()}/api/sync/upload`, {
    method: "PUT",
    headers,
    body: file,
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || "upload failed");
  }
  return res.json() as Promise<FileMeta>;
}

export function openUrl(path: string) {
  return `${apiBase()}/api/open/${encodePath(path)}?token=${encodeURIComponent(getToken())}`;
}

export function downloadUrl(path: string) {
  return `${apiBase()}/api/sync/download/${encodePath(path)}?token=${encodeURIComponent(getToken())}`;
}

export function listGallery(limit = 60, offset = 0) {
  const q = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return request<{ items: FileMeta[] }>(`/api/gallery?${q}`);
}

export function listTrash() {
  return request<{ items: FileMeta[] }>("/api/trash");
}

export function restoreFromTrash(path: string) {
  return request<FileMeta>(`/api/trash/restore/${encodePath(path)}`, { method: "POST" });
}

export function purgeFromTrash(path: string) {
  return request<{ purged: boolean }>(`/api/trash/purge/${encodePath(path)}`, { method: "DELETE" });
}

export function emptyTrash() {
  return request<{ purged: number }>("/api/trash", { method: "DELETE" });
}

function encodePath(path: string) {
  return path
    .split("/")
    .filter(Boolean)
    .map(encodeURIComponent)
    .join("/");
}

export function formatSize(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MB`;
  return `${(n / 1024 ** 3).toFixed(2)} GB`;
}
