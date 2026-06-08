"use client";

import { useState } from "react";
import { Icon } from "@/components/ui";
import { useTheme, Accent, Density, Radius, Theme } from "@/lib/theme";

// Painel de aparência: aplica os tokens configuráveis (data-theme/accent/density/radius).
// Substitui o scaffold de host-editor do handoff por um controle real de usuário.

function Seg<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="col" style={{ gap: 6 }}>
      <span className="field-label">{label}</span>
      <div className="segmented" style={{ width: "100%" }}>
        {options.map((o) => (
          <button key={o.value} className={value === o.value ? "active" : ""} style={{ flex: 1 }} onClick={() => onChange(o.value)}>
            {o.label}
          </button>
        ))}
      </div>
    </div>
  );
}

export function Tweaks() {
  const { theme, setTheme, accent, setAccent, density, setDensity, radius, setRadius } = useTheme();
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        title="Aparência"
        onClick={() => setOpen((o) => !o)}
        className="btn btn-secondary btn-icon"
        style={{ position: "fixed", right: 20, bottom: 20, zIndex: 90, width: 44, height: 44, borderRadius: 99, boxShadow: "var(--shadow-pop)" }}
      >
        <Icon name="sliders" size={18} />
      </button>

      {open && (
        <div
          className="card"
          style={{
            position: "fixed",
            right: 20,
            bottom: 74,
            zIndex: 90,
            width: 280,
            padding: 16,
            display: "flex",
            flexDirection: "column",
            gap: 14,
            boxShadow: "var(--shadow-lg)",
            background: "var(--bg-elevated)",
            animation: "pop .14s ease",
          }}
        >
          <div className="row between center">
            <span className="card-title">Aparência</span>
            <button className="btn btn-ghost btn-icon btn-sm" onClick={() => setOpen(false)}>
              <Icon name="x" size={15} />
            </button>
          </div>
          <Seg<Theme>
            label="Tema"
            value={theme}
            onChange={setTheme}
            options={[
              { value: "dark", label: "Escuro" },
              { value: "light", label: "Claro" },
            ]}
          />
          <Seg<Accent>
            label="Acento"
            value={accent}
            onChange={setAccent}
            options={[
              { value: "blue", label: "Azul" },
              { value: "teal", label: "Teal" },
              { value: "indigo", label: "Índigo" },
            ]}
          />
          <Seg<Density>
            label="Densidade"
            value={density}
            onChange={setDensity}
            options={[
              { value: "compact", label: "Compacto" },
              { value: "default", label: "Padrão" },
              { value: "comfortable", label: "Amplo" },
            ]}
          />
          <Seg<Radius>
            label="Cantos"
            value={radius}
            onChange={setRadius}
            options={[
              { value: "sharp", label: "Reto" },
              { value: "default", label: "Padrão" },
              { value: "round", label: "Arred." },
            ]}
          />
        </div>
      )}
    </>
  );
}
