import { Fragment, ReactNode } from "react";
import { Icon } from "@/components/ui";

export function Topbar({
  title,
  sub,
  actions,
  breadcrumb,
}: {
  title: ReactNode;
  sub?: string;
  actions?: ReactNode;
  breadcrumb?: string[];
}) {
  return (
    <header className="topbar">
      <div className="col" style={{ gap: 2 }}>
        {breadcrumb && (
          <div
            className="row center"
            style={{ gap: 6, fontSize: 12, color: "var(--text-faint)", marginBottom: 1 }}
          >
            {breadcrumb.map((b, i) => (
              <Fragment key={i}>
                {i > 0 && <Icon name="chevronRight" size={12} />}
                <span>{b}</span>
              </Fragment>
            ))}
          </div>
        )}
        <h1 style={{ fontSize: 19, fontWeight: 700 }}>{title}</h1>
        {sub && <div style={{ fontSize: 13, color: "var(--text-tertiary)" }}>{sub}</div>}
      </div>
      <div className="row center" style={{ gap: 9 }}>
        {actions}
      </div>
    </header>
  );
}
