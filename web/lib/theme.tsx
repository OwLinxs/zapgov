"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";

export type Theme = "dark" | "light";
export type Accent = "blue" | "teal" | "indigo";
export type Density = "compact" | "default" | "comfortable";
export type Radius = "sharp" | "default" | "round";

interface ThemeState {
  theme: Theme;
  setTheme: (t: Theme) => void;
  toggleTheme: () => void;
  accent: Accent;
  setAccent: (a: Accent) => void;
  density: Density;
  setDensity: (d: Density) => void;
  radius: Radius;
  setRadius: (r: Radius) => void;
}

const ThemeCtx = createContext<ThemeState | null>(null);

// Lê o valor já aplicado ao <html> pelo ThemeScript (evita mismatch de hidratação).
function attr<T extends string>(name: string, fallback: T): T {
  if (typeof document === "undefined") return fallback;
  return (document.documentElement.getAttribute(name) as T) || fallback;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => attr("data-theme", "dark"));
  const [accent, setAccent] = useState<Accent>(() => attr("data-accent", "blue"));
  const [density, setDensity] = useState<Density>(() => attr("data-density", "default"));
  const [radius, setRadius] = useState<Radius>(() => attr("data-radius", "default"));

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("zg-theme", theme);
  }, [theme]);
  useEffect(() => {
    document.documentElement.setAttribute("data-accent", accent);
    localStorage.setItem("zg-accent", accent);
  }, [accent]);
  useEffect(() => {
    document.documentElement.setAttribute("data-density", density);
    localStorage.setItem("zg-density", density);
  }, [density]);
  useEffect(() => {
    document.documentElement.setAttribute("data-radius", radius);
    localStorage.setItem("zg-radius", radius);
  }, [radius]);

  return (
    <ThemeCtx.Provider
      value={{
        theme,
        setTheme,
        toggleTheme: () => setTheme(theme === "dark" ? "light" : "dark"),
        accent,
        setAccent,
        density,
        setDensity,
        radius,
        setRadius,
      }}
    >
      {children}
    </ThemeCtx.Provider>
  );
}

export function useTheme() {
  const ctx = useContext(ThemeCtx);
  if (!ctx) throw new Error("useTheme deve ser usado dentro de <ThemeProvider>");
  return ctx;
}

// Script inline executado antes do paint: aplica preferências salvas sem flash.
export function ThemeScript() {
  const code = `(function(){try{var d=document.documentElement;
    d.setAttribute('data-theme', localStorage.getItem('zg-theme')||'dark');
    d.setAttribute('data-accent', localStorage.getItem('zg-accent')||'blue');
    d.setAttribute('data-density', localStorage.getItem('zg-density')||'default');
    d.setAttribute('data-radius', localStorage.getItem('zg-radius')||'default');
  }catch(e){}})();`;
  return <script dangerouslySetInnerHTML={{ __html: code }} />;
}
