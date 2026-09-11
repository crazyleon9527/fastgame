import { formatMoney } from "@/utils/money";

/** minor units (×10000) → 主单位展示 */
export function formatMinor(value: number | null | undefined): string {
  if (value == null) return "-";
  return formatMoney(value / 10000);
}

/** RTP ppm → 百分比字符串，960000 → 96.00% */
export function formatRtpPpm(ppm: number | null | undefined): string {
  if (ppm == null) return "-";
  return `${(ppm / 10000).toFixed(2)}%`;
}
