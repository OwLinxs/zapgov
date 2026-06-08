import { InputHTMLAttributes, ReactNode } from "react";
import { Icon } from "./Icon";
import { IconName } from "@/lib/icons";

export function Field({
  label,
  hint,
  error,
  children,
}: {
  label?: ReactNode;
  hint?: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="field">
      {label && <label className="field-label">{label}</label>}
      {children}
      {error ? (
        <span className="field-error">
          <Icon name="alertCircle" size={13} />
          {error}
        </span>
      ) : hint ? (
        <span className="field-hint">{hint}</span>
      ) : null}
    </div>
  );
}

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  icon?: IconName;
  affix?: string;
  error?: boolean;
}

export function Input({ icon, affix, error, ...rest }: InputProps) {
  const cls = "input" + (error ? " has-error" : "");
  if (!icon && !affix) return <input className={cls} {...rest} />;
  return (
    <div className="input-wrap">
      {icon && (
        <span className="icon-lead">
          <Icon name={icon} size={16} />
        </span>
      )}
      <input className={cls} {...rest} />
      {affix && <span className="input-affix">{affix}</span>}
    </div>
  );
}
