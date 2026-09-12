import { http } from "@/utils/http";

export type PageResult<T> = { total: number; list: T[] };

export const loginApi = (data: {
  username: string;
  password: string;
  totpCode?: string;
  recoveryCode?: string;
}) => http.post<any, any>("/login", { data });

export const listMerchants = (page = 1, pageSize = 20) =>
  http.get<any, any>("/merchants", { params: { page, pageSize } });

export const createMerchant = (data: object) =>
  http.post<any, any>("/merchants", { data });

export const updateMerchant = (id: number, data: object) =>
  http.request<any>("put", `/merchants/${id}`, { data });

export const rotateMerchantKey = (id: number, gracePeriodHours = 24) =>
  http.post<any, any>(`/merchants/${id}/rotate-key`, {
    data: { gracePeriodHours }
  });

export const getMerchantAllowedIPs = (id: number) =>
  http.get<any, any>(`/merchants/${id}/allowed-ips`);

export const updateMerchantAllowedIPs = (id: number, allowedIps: string[]) =>
  http.request<any>("put", `/merchants/${id}/allowed-ips`, {
    data: { allowedIps }
  });

export const listBlacklist = (params: object) =>
  http.get<any, any>("/risk-blacklist", { params });

export const createBlacklist = (data: object) =>
  http.post<any, any>("/risk-blacklist", { data });

export const deleteBlacklist = (id: number) =>
  http.request<any>("delete", `/risk-blacklist/${id}`);

export const listGameConfigs = (merchantId: number, gameCode = "") =>
  http.get<any, any>("/game-configs", { params: { merchantId, gameCode } });

export const upsertGameConfig = (data: object) =>
  http.post<any, any>("/game-configs", { data });

export const deleteGameConfig = (id: number) =>
  http.request<any>("delete", `/game-configs/${id}`);

export const getRtpReport = (params: object) =>
  http.get<any, any>("/reports/rtp", { params });

export const listDailySettlements = (params: object) =>
  http.get<any, any>("/reports/daily-settlements", { params });

export const syncDailySettlements = (data: object) =>
  http.post<any, any>("/reports/daily-settlements/sync", { data });

export const confirmDailySettlement = (id: number) =>
  http.post<any, any>(`/reports/daily-settlements/${id}/confirm`);

export const listRiskAlerts = (limit = 50) =>
  http.get<any, any>("/risk-alerts", { params: { limit } });

export const ackRiskAlert = (id: number) =>
  http.post<any, any>(`/risk-alerts/${id}/ack`);

export const lookupTrace = (traceId: string) =>
  http.get<any, any>(`/traces/${encodeURIComponent(traceId)}`);

export const lookupTraceByRound = (roundId: string) =>
  http.get<any, any>(`/traces/by-round/${encodeURIComponent(roundId)}`);

export const listWalletBreakers = () =>
  http.get<any, any>("/wallet/breakers");

export const resetWalletBreaker = (merchantCode: string) =>
  http.post<any, any>("/wallet/breaker/reset", { data: { merchantCode } });

export const listAdminUsers = (page = 1, pageSize = 20) =>
  http.get<any, any>("/users", { params: { page, pageSize } });

export const createAdminUser = (data: object) =>
  http.post<any, any>("/users", { data });

export const updateAdminUser = (id: number, data: object) =>
  http.request<any>("put", `/users/${id}`, { data });

export const listRoles = () => http.get<any, any>("/roles");

export const totpSetup = () => http.post<any, any>("/totp/setup");

export const totpConfirm = (totpCode: string) =>
  http.post<any, any>("/totp/confirm", { data: { totpCode } });

export const listLocales = () => http.get<any, any>("/locales");

export const getI18nDictionary = (bundle = "ui.admin", locale = "zh-CN") =>
  http.get<any, any>("/i18n/dictionary", { params: { bundle, locale } });

export const listGameCategories = () =>
  http.get<any, any>("/platform/game-categories");

export const listPlatformGames = (params: object) =>
  http.get<any, any>("/platform/games", { params });

export const createPlatformGame = (data: object) =>
  http.post<any, any>("/platform/games", { data });

export const updatePlatformGame = (id: number, data: object) =>
  http.request<any>("put", `/platform/games/${id}`, { data });

export const listGameRtpTiers = (gameId: number) =>
  http.get<any, any>(`/platform/games/${gameId}/rtp-tiers`);

export const listMerchantGames = (merchantId: number, page = 1, pageSize = 50) =>
  http.get<any, any>("/merchant-games", {
    params: { merchantId, page, pageSize }
  });

export const createMerchantGame = (data: object) =>
  http.post<any, any>("/merchant-games", { data });

export const updateMerchantGame = (id: number, data: object) =>
  http.request<any>("put", `/merchant-games/${id}`, { data });

export const deleteMerchantGame = (id: number) =>
  http.request<any>("delete", `/merchant-games/${id}`);

export const listAuditLogs = (params: object) =>
  http.get<any, any>("/audit-logs", { params });
