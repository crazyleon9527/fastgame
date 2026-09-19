const Layout = () => import("@/layout/index.vue");

export default [
  {
    path: "/login",
    name: "Login",
    component: () => import("@/views/login/index.vue"),
    meta: {
      title: "登录",
      showLink: false
    }
  },
  // 全屏403（无权访问）页面
  {
    path: "/access-denied",
    name: "AccessDenied",
    component: () => import("@/views/error/403.vue"),
    meta: {
      title: "403",
      showLink: false
    }
  },
  // 纯 redirect，没有页面也没有 meta。
  // 这里显式给出 meta.showLink:false 而不是省略 meta：constantMenus 会把
  // remainingRouter 一起交给 ascending() 排序，而 ascending 对"没有 meta"的路由
  // 会补一个空 meta 并写 rank —— 显式声明能保证它永远不会被当成菜单项渲染出来。
  {
    path: "/error/403",
    redirect: "/access-denied",
    meta: {
      showLink: false
    }
  },
  {
    path: "/error/404",
    name: "NotFound",
    component: () => import("@/views/error/404.vue"),
    meta: {
      title: "404",
      showLink: false
    }
  },
  // 全屏500（服务器出错）页面
  {
    path: "/server-error",
    name: "ServerError",
    component: () => import("@/views/error/500.vue"),
    meta: {
      title: "500",
      showLink: false
    }
  },
  {
    path: "/redirect",
    component: Layout,
    meta: {
      title: "加载中...",
      showLink: false
    },
    children: [
      {
        path: "/redirect/:path(.*)",
        name: "Redirect",
        component: () => import("@/layout/redirect.vue")
      }
    ]
  },
  // 通配兜底路由：同上，显式 meta.showLink:false，避免被 ascending 补出 rank 后
  // 当成菜单项（它连标题都没有）
  {
    path: "/:pathMatch(.*)*",
    redirect: "/error/404",
    meta: {
      showLink: false
    }
  }
] satisfies Array<RouteConfigsTable>;
