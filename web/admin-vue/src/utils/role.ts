import { storageLocal } from "@pureadmin/utils";
import { userKey, type DataInfo } from "@/utils/auth";

export function currentRole(): string {
  const info = storageLocal().getItem<DataInfo<number>>(userKey);
  return info?.roles?.[0] ?? "viewer";
}

export function canWrite(): boolean {
  return currentRole() !== "viewer";
}

export function isAdmin(): boolean {
  return currentRole() === "admin";
}
