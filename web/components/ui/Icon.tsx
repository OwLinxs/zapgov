import { CSSProperties } from "react";
import { ICONS, IconName } from "@/lib/icons";

export function Icon({
  name,
  size = 16,
  stroke = 1.75,
  fill = false,
  style,
  className,
}: {
  name: IconName;
  size?: number;
  stroke?: number;
  fill?: boolean;
  style?: CSSProperties;
  className?: string;
}) {
  const Cmp = ICONS[name];
  if (!Cmp) return <span className="icon" style={{ width: size, height: size, ...style }} />;
  return (
    <span className={"icon " + (className || "")} style={style} aria-hidden="true">
      <Cmp size={size} strokeWidth={stroke} fill={fill ? "currentColor" : "none"} />
    </span>
  );
}
