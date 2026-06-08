"use client";

import { ReactNode, useEffect } from "react";
import { Icon } from "./Icon";
import { Button } from "./Button";
import { IconName } from "@/lib/icons";

export function Modal({
  open,
  onClose,
  children,
  width = 460,
}: {
  open: boolean;
  onClose?: () => void;
  children: ReactNode;
  width?: number;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose?.();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div
      className="overlay"
      onMouseDown={(e) => e.target === e.currentTarget && onClose?.()}
    >
      <div className="modal" style={{ maxWidth: width }}>
        {children}
      </div>
    </div>
  );
}

export function ConfirmModal({
  open,
  onClose,
  onConfirm,
  title,
  message,
  confirmLabel = "Confirmar",
  danger,
  icon = "alert",
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message?: string;
  confirmLabel?: string;
  danger?: boolean;
  icon?: IconName;
}) {
  return (
    <Modal open={open} onClose={onClose} width={420}>
      <div className="modal-body">
        <div
          className="modal-icon"
          style={{
            background: danger ? "var(--danger-soft)" : "var(--brand-soft)",
            color: danger ? "var(--st-failed)" : "var(--brand-text)",
          }}
        >
          <Icon name={icon} size={21} />
        </div>
        <div className="modal-title">{title}</div>
        {message && (
          <div className="modal-sub" style={{ marginTop: 6, lineHeight: 1.5 }}>
            {message}
          </div>
        )}
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>
          Cancelar
        </Button>
        <Button variant={danger ? "danger" : "primary"} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </Modal>
  );
}
