import { loginApi } from "@/api/fastgame";
import { http } from "@/utils/http";
import { setTotpPending } from "@/utils/totp";

export type UserResult = {
  success: boolean;
  data: {
    avatar: string;
    username: string;
    nickname: string;
    roles: Array<string>;
    permissions: Array<string>;
    accessToken: string;
    refreshToken: string;
    expires: Date | string;
    requiresTotpSetup?: boolean;
  };
};

export type RefreshTokenResult = {
  success: boolean;
  data: {
    accessToken: string;
    refreshToken: string;
    expires: Date | string;
  };
};

function mapPermissions(roleName: string) {
  if (roleName === "admin") return ["*:*:*"];
  if (roleName === "operator") return ["fastgame:write"];
  return ["fastgame:read"];
}

/** 登录 — 对接 FastGame admin-api */
export const getLogin = async (data?: {
  username: string;
  password: string;
  totpCode?: string;
  recoveryCode?: string;
}): Promise<UserResult> => {
  const raw = await loginApi({
    username: data?.username ?? "",
    password: data?.password ?? "",
    totpCode: data?.totpCode || undefined,
    recoveryCode: data?.recoveryCode || undefined
  });
  const expireMs =
    raw.expireAt > 1e12 ? raw.expireAt : (raw.expireAt ?? 0) * 1000;
  const expires = new Date(expireMs || Date.now() + 86400000);
  const roleName = raw.roleName || "viewer";
  setTotpPending(!!raw.requiresTotpSetup);
  return {
    success: true,
    data: {
      avatar: "",
      username: data?.username ?? "",
      nickname: data?.username ?? "",
      roles: [roleName],
      permissions: mapPermissions(roleName),
      accessToken: raw.accessToken,
      refreshToken: raw.accessToken,
      expires,
      requiresTotpSetup: raw.requiresTotpSetup
    }
  };
};

/** FastGame 暂无 refresh-token，复用 accessToken */
export const refreshTokenApi = (data?: { refreshToken: string }) => {
  return Promise.resolve({
    success: true,
    data: {
      accessToken: data?.refreshToken ?? "",
      refreshToken: data?.refreshToken ?? "",
      expires: new Date(Date.now() + 86400000)
    }
  } as RefreshTokenResult);
};
