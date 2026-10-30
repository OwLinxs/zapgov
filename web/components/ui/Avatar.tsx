export function initials(name: string): string {
  return name
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0])
    .join("")
    .toUpperCase();
}

// Equipe usa brand-soft; cidadão usa o tom neutro (design system → Avatar).
export function Avatar({
  name,
  size = 36,
  citizen,
  presence,
}: {
  name: string;
  size?: number;
  citizen?: boolean;
  presence?: "on" | "away" | "off";
}) {
  const safe = name || "?";
  return (
    <div
      className={"avatar" + (citizen ? " citizen" : "")}
      style={{ width: size, height: size, fontSize: Math.round(size * 0.36) }}
      title={safe}
    >
      {initials(safe)}
      {presence && (
        <span
          className={"zg-presence " + (presence === "off" ? "" : presence)}
          style={{ position: "absolute", right: -1, bottom: -1, width: 10, height: 10, borderRadius: "50%", border: "2px solid var(--presence-ring, var(--bg-surface))" }}
        />
      )}
    </div>
  );
}
