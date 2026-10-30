"use client";

import { createContext, useCallback, useContext, useState, ReactNode } from "react";
import { Icon } from "./Icon";
import { IconName } from "@/lib/icons";

export type ToastType = "success" | "error" | "info" | "wa";

export interface ToastInput {
  type?: ToastType;
  title: string;
  message?: string;
  duration?: number;
}

interface ToastItem extends ToastInput {
  id: string;
}

const ToastCtx = createContext<(t: ToastInput) => void>(() => {});

const ICO: Record<ToastType, { i: IconName; c: string }> = {
  success: { i: "checkCircle", c: "var(--st-read)" },
  error: { i: "alertCircle", c: "var(--st-failed)" },
  info: { i: "info", c: "var(--brand-text)" },
  wa: { i: "message", c: "var(--wa-deep)" },
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const push = useCallback((t: ToastInput) => {
    const id = Math.random().toString(36).slice(2);
    setToasts((ts) => [...ts, { id, ...t }]);
    setTimeout(() => setToasts((ts) => ts.filter((x) => x.id !== id)), t.duration || 3600);
  }, []);

  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div className="toast-region">
        {toasts.map((t) => {
          const ic = ICO[t.type || "info"];
          return (
            <div key={t.id} className="toast" role={t.type === "error" ? "alert" : "status"}>
              <div className="toast-ico" style={{ color: ic.c }}>
                <Icon name={ic.i} size={20} />
              </div>
              <div className="grow">
                <div className="toast-title">{t.title}</div>
                {t.message && <div className="toast-msg">{t.message}</div>}
              </div>
              <button
                className="btn btn-ghost btn-icon btn-sm"
                aria-label="Fechar aviso"
                onClick={() => setToasts((ts) => ts.filter((x) => x.id !== t.id))}
              >
                <Icon name="x" size={14} />
              </button>
            </div>
          );
        })}
      </div>
    </ToastCtx.Provider>
  );
}

export function useToast() {
  return useContext(ToastCtx);
}
