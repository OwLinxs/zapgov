import { Fragment, ReactNode } from "react";
import { Icon } from "./Icon";
import { STATUS } from "./StatusBadge";

// Destaca variáveis {{1}} ou [texto] no corpo da mensagem.
export function highlightVars(text: string): ReactNode[] {
  const parts = String(text).split(/(\{\{\d+\}\}|\[[^\]]+\])/g);
  return parts.map((p, i) =>
    /^\{\{\d+\}\}$|^\[[^\]]+\]$/.test(p) ? (
      <span key={i} className="wa-var">
        {p}
      </span>
    ) : (
      <Fragment key={i}>{p}</Fragment>
    ),
  );
}

export function WaBubble({
  text,
  dir = "out",
  time = "14:32",
  status = "lido",
  buttons,
  highlight,
}: {
  text: string;
  dir?: "in" | "out";
  time?: string;
  status?: string;
  buttons?: string[];
  highlight?: boolean;
}) {
  const st = STATUS[status];
  return (
    <div className={"wa-row " + dir}>
      <div className={"wa-bubble " + dir}>
        <div>{highlight ? highlightVars(text) : text}</div>
        {buttons && (
          <div className="wa-btns">
            {buttons.map((b, i) => (
              <div key={i} className="wa-btn">
                {b}
              </div>
            ))}
          </div>
        )}
        <div className="wa-meta">
          {time}
          {dir === "out" && st && (
            <Icon
              name={st.icon === "checkDouble" ? "checkDouble" : "check"}
              size={14}
              style={{ color: status === "lido" ? "var(--st-read)" : "inherit" }}
            />
          )}
        </div>
      </div>
    </div>
  );
}
