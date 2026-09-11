import { ElMessage, ElMessageBox } from "element-plus";

/** 统一 API 错误提示，失败时返回 null */
export async function withRequest<T>(
  fn: () => Promise<T>,
  errMsg = "请求失败",
  silent = false
): Promise<T | null> {
  try {
    return await fn();
  } catch (err: any) {
    if (!silent) ElMessage.error(err?.message || errMsg);
    return null;
  }
}

/** 确认对话框，取消时不抛异常、不写控制台 */
export async function confirmAction(
  message: string,
  title = "提示",
  options?: Record<string, unknown>
): Promise<boolean> {
  try {
    await ElMessageBox.confirm(message, title, options);
    return true;
  } catch {
    return false;
  }
}
