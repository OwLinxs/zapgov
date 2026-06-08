import { ButtonHTMLAttributes, ReactNode } from "react";
import { Icon } from "./Icon";
import { IconName } from "@/lib/icons";

type Variant = "primary" | "wa" | "secondary" | "ghost" | "danger" | "danger-ghost";
type Size = "sm" | "default" | "lg";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  icon?: IconName;
  iconRight?: IconName;
  children?: ReactNode;
}

export function Button({
  variant = "secondary",
  size = "default",
  icon,
  iconRight,
  children,
  className = "",
  ...rest
}: ButtonProps) {
  const cls = [
    "btn",
    "btn-" + variant,
    size === "sm" ? "btn-sm" : size === "lg" ? "btn-lg" : "",
    !children ? "btn-icon" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");
  const isz = size === "sm" ? 14 : 16;
  return (
    <button className={cls} {...rest}>
      {icon && <Icon name={icon} size={isz} />}
      {children}
      {iconRight && <Icon name={iconRight} size={isz} />}
    </button>
  );
}
