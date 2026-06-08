"use client";

import { ReactNode, useEffect, useRef, useState } from "react";

export function Dropdown({
  trigger,
  children,
  align = "right",
  up = false,
}: {
  trigger: ReactNode;
  children: ReactNode;
  align?: "left" | "right";
  up?: boolean; // abre para cima (ex: menu no rodapé da sidebar)
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("mousedown", onClick);
    return () => window.removeEventListener("mousedown", onClick);
  }, [open]);

  return (
    <div ref={ref} style={{ position: "relative" }}>
      <div onClick={() => setOpen((o) => !o)}>{trigger}</div>
      {open && (
        <div
          className="dropdown"
          style={{ [up ? "bottom" : "top"]: "calc(100% + 5px)", [align]: 0 }}
          onClick={() => setOpen(false)}
        >
          {children}
        </div>
      )}
    </div>
  );
}
