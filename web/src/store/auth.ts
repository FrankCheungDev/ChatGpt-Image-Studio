"use client";

import localforage from "localforage";

export type CurrentUser = {
  id: string;
  username: string;
  role: "admin" | "user";
  disabled?: boolean;
};

export const AUTH_KEY_STORAGE_KEY = "chatgpt2api_auth_key";
export const CURRENT_USER_STORAGE_KEY = "chatgpt2api_current_user";

const authStorage = localforage.createInstance({
  name: "chatgpt2api",
  storeName: "auth",
});

export async function getStoredAuthKey() {
  return "";
}

export async function getStoredUser() {
  if (typeof window === "undefined") {
    return null;
  }
  const value = await authStorage.getItem<CurrentUser>(CURRENT_USER_STORAGE_KEY);
  return value ?? null;
}

export async function setStoredUser(user: CurrentUser | null) {
  if (!user) {
    await clearStoredAuthKey();
    return;
  }
  await authStorage.setItem(CURRENT_USER_STORAGE_KEY, user);
}

export async function setStoredAuthKey(_authKey: string) {
  await clearStoredAuthKey();
}

export async function clearStoredAuthKey() {
  if (typeof window === "undefined") {
    return;
  }
  await authStorage.removeItem(AUTH_KEY_STORAGE_KEY);
  await authStorage.removeItem(CURRENT_USER_STORAGE_KEY);
}
