/** 金额展示（后端 daily settlement 等为 float64 主单位） */
export function formatMoney(
  value: number | string | null | undefined,
  decimals = 2
): string {
  if (value == null || value === "") return "-";
  const n = Number(value);
  if (Number.isNaN(n)) return String(value);
  return n.toLocaleString("zh-CN", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals
  });
}
