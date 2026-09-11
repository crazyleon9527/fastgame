const Layout = () => import("@/layout/index.vue");

export default {
  path: "/",
  name: "Home",
  component: Layout,
  redirect: "/dashboard",
  meta: {
    icon: "ep/home-filled",
    title: "控制台",
    rank: 0
  },
  children: [
    {
      path: "/dashboard",
      name: "Dashboard",
      component: () => import("@/views/fastgame/dashboard/index.vue"),
      meta: {
        title: "控制台",
        i18nKey: "nav.dashboard",
        roles: ["admin", "operator", "viewer"]
      }
    }
  ]
} satisfies RouteConfigsTable;
