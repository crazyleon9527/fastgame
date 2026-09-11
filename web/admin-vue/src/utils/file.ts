import Axios from "axios";
import { getToken, formatToken } from "@/utils/auth";
import { useUserStoreHook } from "@/store/modules/user";

const API_BASE = import.meta.env.VITE_API_BASE || "/api/v1/admin";

function authHeaders() {
  const token = getToken()?.accessToken;
  return token ? { Authorization: formatToken(token) } : {};
}

function handleFileError(err: any): never {
  const status = err?.response?.status;
  const data = err?.response?.data;
  let msg =
    typeof data === "string"
      ? data
      : (data as { message?: string })?.message;
  if (status === 401) {
    useUserStoreHook().logOut();
    throw new Error("登录已过期，请重新登录");
  }
  throw new Error(msg || err?.message || "请求失败");
}

export async function downloadExcel(
  path: string,
  params?: Record<string, unknown>,
  fallbackName = "export.xlsx"
) {
  try {
    const res = await Axios.get(`${API_BASE}${path}`, {
      params,
      responseType: "blob",
      headers: authHeaders()
    });
    const disposition = String(res.headers["content-disposition"] || "");
    const match = /filename="([^"]+)"/.exec(disposition);
    const name = match?.[1] ?? fallbackName;
    const url = URL.createObjectURL(new Blob([res.data]));
    const link = document.createElement("a");
    link.href = url;
    link.download = name;
    link.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    handleFileError(err);
  }
}

export async function uploadAsset(file: File) {
  try {
    const fd = new FormData();
    fd.append("file", file);
    const res = await Axios.post(`${API_BASE}/files/upload`, fd, {
      headers: {
        ...authHeaders(),
        "Content-Type": "multipart/form-data"
      }
    });
    return res.data as { url: string; filename: string; size: number };
  } catch (err) {
    handleFileError(err);
  }
}

export async function importExcel(path: string, file: File) {
  try {
    const fd = new FormData();
    fd.append("file", file);
    const res = await Axios.post(`${API_BASE}${path}`, fd, {
      headers: {
        ...authHeaders(),
        "Content-Type": "multipart/form-data"
      }
    });
    return res.data as { imported: number };
  } catch (err) {
    handleFileError(err);
  }
}
