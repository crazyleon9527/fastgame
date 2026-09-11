/** FastGame 使用前端静态路由，不请求后端动态菜单 */
export const getAsyncRoutes = () => {
  return Promise.resolve({ success: true, data: [] as Array<any> });
};
