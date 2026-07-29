"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { useRouter } from "next/navigation";
import { authMe, clearSession, getToken, type AuthUser } from "@/lib/api";

interface AuthState {
  user: AuthUser | null;
  setSession: (token: string, user: AuthUser) => void;
  logout: () => void;
}

const Ctx = createContext<AuthState | null>(null);

function readUser(): AuthUser | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = localStorage.getItem("zg-user");
    return raw ? (JSON.parse(raw) as AuthUser) : null;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(() => readUser());

  const setSession = (token: string, u: AuthUser) => {
    localStorage.setItem("zg-token", token);
    localStorage.setItem("zg-user", JSON.stringify(u));
    setUser(u);
  };

  // Revalida a sessão ao abrir o app: traz módulos/papel atualizados sem relogar
  // (ex.: superadmin ligou/desligou um módulo do órgão).
  useEffect(() => {
    if (!getToken()) return;
    authMe()
      .then((u) => {
        localStorage.setItem("zg-user", JSON.stringify(u));
        setUser(u);
      })
      .catch(() => { /* token inválido → AuthGate cuida do redirect */ });
  }, []);

  const logout = () => {
    clearSession();
    setUser(null);
    if (typeof window !== "undefined") location.href = "/login";
  };

  return <Ctx.Provider value={{ user, setSession, logout }}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useAuth fora do AuthProvider");
  return ctx;
}

// Guard: bloqueia o app sem token (redireciona ao login).
export function AuthGate({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [ok, setOk] = useState(false);

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login");
    } else {
      setOk(true);
    }
  }, [router]);

  if (!ok) return null;
  return <>{children}</>;
}
