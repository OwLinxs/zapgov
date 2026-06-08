import { CSSProperties } from "react";
import { ICON_PATHS, IconName } from "@/lib/icons";

export function Icon({
  name,
  size = 18,
  stroke = 1.9,
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
  const def = ICON_PATHS[name];
  if (!def) return <span className="icon" style={{ width: size, height: size, ...style }} />;
  const paths = def.split("|");
  return (
    <span className={"icon " + (className || "")} style={style}>
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill={fill ? "currentColor" : "none"}
        stroke={fill ? "none" : "currentColor"}
        strokeWidth={stroke}
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        {paths.map((d, i) => (
          <path key={i} d={d} />
        ))}
      </svg>
    </span>
  );
}
