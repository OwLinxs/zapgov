export function Logo({ size = 30, showText = true }: { size?: number; showText?: boolean }) {
  return (
    <div className="row center" style={{ gap: 10 }}>
      <div
        style={{
          width: size,
          height: size,
          borderRadius: size * 0.28,
          background: "var(--brand)",
          display: "grid",
          placeItems: "center",
          flex: "none",
          position: "relative",
          overflow: "hidden",
        }}
      >
        <svg width={size * 0.6} height={size * 0.6} viewBox="0 0 24 24" fill="none">
          <path
            d="M13 2 4 13h7l-1 9 10-12h-7l1-8z"
            fill="var(--on-brand)"
            stroke="var(--on-brand)"
            strokeWidth={1}
            strokeLinejoin="round"
          />
        </svg>
      </div>
      {showText && (
        <div className="col" style={{ gap: 0, lineHeight: 1.1 }}>
          <div style={{ fontWeight: 700, fontSize: 16, letterSpacing: "-0.01em" }}>ZapGov</div>
          <div
            style={{
              fontSize: 9.5,
              fontWeight: 600,
              color: "var(--text-tertiary)",
              letterSpacing: "0.08em",
              textTransform: "uppercase",
            }}
          >
            Comunicação Oficial
          </div>
        </div>
      )}
    </div>
  );
}
