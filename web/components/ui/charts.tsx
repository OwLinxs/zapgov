"use client";

import { useState } from "react";

interface BarDatum {
  [k: string]: string | number;
}

export function BarChart({
  data,
  height = 150,
  color = "var(--brand)",
  labelKey = "label",
  valueKey = "value",
  secondaryKey,
}: {
  data: BarDatum[];
  height?: number;
  color?: string;
  labelKey?: string;
  valueKey?: string;
  secondaryKey?: string;
}) {
  const max = Math.max(...data.map((d) => Number(d[valueKey])), 1);
  const [hover, setHover] = useState<number | null>(null);
  return (
    <div style={{ position: "relative" }}>
      <div className="row" style={{ alignItems: "flex-end", gap: 5, height }}>
        {data.map((d, i) => {
          const pct = (Number(d[valueKey]) / max) * 100;
          const sec = secondaryKey ? (Number(d[secondaryKey]) / max) * 100 : 0;
          return (
            <div
              key={i}
              className="col"
              style={{
                flex: 1,
                height: "100%",
                justifyContent: "flex-end",
                alignItems: "center",
                gap: 6,
                cursor: "pointer",
              }}
              onMouseEnter={() => setHover(i)}
              onMouseLeave={() => setHover(null)}
            >
              {hover === i && (
                <div
                  style={{
                    position: "absolute",
                    bottom: height + 6,
                    background: "var(--bg-elevated)",
                    border: "1px solid var(--border)",
                    borderRadius: 7,
                    padding: "6px 9px",
                    fontSize: 11.5,
                    whiteSpace: "nowrap",
                    boxShadow: "var(--shadow-pop)",
                    zIndex: 5,
                    fontWeight: 600,
                  }}
                >
                  {String(d[labelKey])}:{" "}
                  <span className="mono">{Number(d[valueKey]).toLocaleString("pt-BR")}</span>
                </div>
              )}
              <div
                style={{
                  width: "100%",
                  maxWidth: 26,
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  justifyContent: "flex-end",
                  borderRadius: 4,
                  overflow: "hidden",
                  background: "var(--bg-sunken)",
                }}
              >
                {secondaryKey && (
                  <div
                    style={{
                      height: sec + "%",
                      background: "var(--st-read)",
                      opacity: hover === i ? 1 : 0.85,
                      transition: "all .15s",
                    }}
                  />
                )}
                <div
                  style={{
                    height: pct - sec + "%",
                    background: color,
                    opacity: hover === i ? 1 : 0.85,
                    transition: "all .15s",
                  }}
                />
              </div>
              <div style={{ fontSize: 10, color: "var(--text-faint)", fontFamily: "JetBrains Mono" }}>
                {String(d[labelKey])}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function Donut({
  value,
  size = 90,
  stroke = 9,
  color = "var(--st-read)",
  label,
}: {
  value: number;
  size?: number;
  stroke?: number;
  color?: string;
  label?: string;
}) {
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  return (
    <div style={{ position: "relative", width: size, height: size }}>
      <svg width={size} height={size} style={{ transform: "rotate(-90deg)" }}>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--bg-active)" strokeWidth={stroke} />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke={color}
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={c * (1 - value / 100)}
          style={{ transition: "stroke-dashoffset .6s cubic-bezier(.2,.8,.3,1)" }}
        />
      </svg>
      <div style={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 1, lineHeight: 1 }}>
        <div className="mono" style={{ fontSize: size * 0.22, fontWeight: 700 }}>
          {value.toFixed(1)}%
        </div>
        {label && <div style={{ fontSize: 10, color: "var(--text-faint)" }}>{label}</div>}
      </div>
    </div>
  );
}

export function Spark({
  data,
  width = 100,
  height = 30,
  color = "var(--brand)",
}: {
  data: number[];
  width?: number;
  height?: number;
  color?: string;
}) {
  const max = Math.max(...data);
  const min = Math.min(...data);
  const range = max - min || 1;
  const pts = data.map((d, i) => [
    (i / (data.length - 1)) * width,
    height - ((d - min) / range) * (height - 4) - 2,
  ]);
  const path = pts.map((p, i) => (i ? "L" : "M") + p[0].toFixed(1) + " " + p[1].toFixed(1)).join(" ");
  const area = path + ` L${width} ${height} L0 ${height} Z`;
  const id = "sp" + Math.random().toString(36).slice(2, 7);
  return (
    <svg width={width} height={height} style={{ display: "block" }}>
      <defs>
        <linearGradient id={id} x1={0} y1={0} x2={0} y2={1}>
          <stop offset="0%" stopColor={color} stopOpacity={0.25} />
          <stop offset="100%" stopColor={color} stopOpacity={0} />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${id})`} />
      <path
        d={path}
        fill="none"
        stroke={color}
        strokeWidth={1.8}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
