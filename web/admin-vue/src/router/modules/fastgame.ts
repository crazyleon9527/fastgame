const Layout = () => import("@/layout/index.vue");

export default [
  {
    path: "/risk",
    name: "Risk",
    component: Layout,
    redirect: "/risk/blacklist",
    meta: {
      icon: "ep/warning-filled",
      title: "风控中心",
      rank: 1
    },
    children: [
      {
        path: "/risk/blacklist",
        name: "RiskBlacklist",
        component: () => import("@/views/fastgame/blacklist/index.vue"),
        meta: {
          title: "风控黑名单",
          i18nKey: "nav.blacklist",
          roles: ["admin", "operator", "viewer"]
        }
      },
      {
        path: "/risk/alerts",
        name: "RiskAlerts",
        component: () => import("@/views/fastgame/alerts/index.vue"),
        meta: {
          title: "RTP 告警",
          i18nKey: "nav.alerts",
          roles: ["admin", "operator", "viewer"]
        }
      }
    ]
  },
  {
    path: "/merchant",
    name: "Merchant",
    component: Layout,
    redirect: "/merchant/list",
    meta: {
      icon: "ep/office-building",
      title: "商户运营",
      rank: 2
    },
    children: [
      {
        path: "/merchant/list",
        name: "MerchantList",
        component: () => import("@/views/fastgame/merchants/index.vue"),
        meta: {
          title: "商户管理",
          i18nKey: "nav.merchants",
          roles: ["admin", "operator", "viewer"]
        }
      },
      {
        path: "/merchant/games",
        name: "MerchantGames",
        component: () => import("@/views/fastgame/merchant/games/index.vue"),
        meta: {
          title: "游戏大厅",
          i18nKey: "nav.merchant_games",
          roles: ["admin", "operator", "viewer"]
        }
      },
      {
        path: "/merchant/whitelist",
        name: "MerchantWhitelist",
        component: () => import("@/views/fastgame/whitelist/index.vue"),
        meta: {
          title: "IP 白名单",
          i18nKey: "nav.whitelist",
          roles: ["admin", "operator", "viewer"]
        }
      }
    ]
  },
  {
    path: "/game",
    name: "Game",
    component: Layout,
    redirect: "/game/config",
    meta: {
      icon: "ep/video-play",
      title: "游戏配置",
      rank: 3
    },
    children: [
      {
        path: "/game/platform",
        name: "PlatformGames",
        component: () => import("@/views/fastgame/platform/games/index.vue"),
        meta: {
          title: "平台游戏目录",
          i18nKey: "nav.platform_games",
          roles: ["admin", "operator", "viewer"]
        }
      },
      {
        path: "/game/config",
        name: "GameConfig",
        component: () => import("@/views/fastgame/gameconfig/index.vue"),
        meta: {
          title: "参数配置",
          i18nKey: "nav.game_config",
          roles: ["admin", "operator", "viewer"]
        }
      }
    ]
  },
  {
    path: "/finance",
    name: "Finance",
    component: Layout,
    redirect: "/finance/settlement",
    meta: {
      icon: "ep/data-analysis",
      title: "财务对账",
      rank: 4
    },
    children: [
      {
        path: "/finance/settlement",
        name: "DailySettlement",
        component: () => import("@/views/fastgame/settlement/index.vue"),
        meta: {
          title: "日结算",
          i18nKey: "nav.settlements",
          roles: ["admin", "operator", "viewer"]
        }
      },
      {
        path: "/finance/rtp",
        name: "RtpReport",
        component: () => import("@/views/fastgame/rtpreport/index.vue"),
        meta: {
          title: "RTP 报表",
          i18nKey: "nav.rtp",
          roles: ["admin", "operator", "viewer"]
        }
      }
    ]
  },
  {
    path: "/ops",
    name: "Ops",
    component: Layout,
    redirect: "/ops/trace",
    meta: {
      icon: "ep/search",
      title: "运维工具",
      rank: 5
    },
    children: [
      {
        path: "/ops/trace",
        name: "TraceLookup",
        component: () => import("@/views/fastgame/trace/index.vue"),
        meta: {
          title: "Trace 追踪",
          i18nKey: "nav.trace",
          roles: ["admin", "operator", "viewer"]
        }
      }
    ]
  },
  {
    path: "/system",
    name: "System",
    component: Layout,
    redirect: "/system/users",
    meta: {
      icon: "ep/setting",
      title: "系统管理",
      rank: 6
    },
    children: [
      {
        path: "/system/users",
        name: "AdminUsers",
        component: () => import("@/views/fastgame/users/index.vue"),
        meta: {
          title: "管理员",
          i18nKey: "nav.users",
          roles: ["admin"]
        }
      },
      {
        path: "/system/audit",
        name: "AuditLogs",
        component: () => import("@/views/fastgame/audit/index.vue"),
        meta: {
          title: "审计日志",
          i18nKey: "nav.audit",
          roles: ["admin", "operator", "viewer"]
        }
      }
    ]
  }
] satisfies Array<RouteConfigsTable>;
