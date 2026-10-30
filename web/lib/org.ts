"use client";

import { useEffect, useState } from "react";
import { getMeta, getOrg } from "@/lib/api";

// Nome do órgão e estado da conexão Meta, buscados uma vez e compartilhados
// pelo shell (sidebar + topbar).
let orgP: Promise<string> | null = null;
let connP: Promise<"ok" | "err"> | null = null;

export function useOrgName(): string {
  const [nome, setNome] = useState("");
  useEffect(() => {
    orgP ??= getOrg()
      .then((o) => o.nome)
      .catch(() => {
        orgP = null;
        return "";
      });
    orgP.then(setNome);
  }, []);
  return nome;
}

export function useMetaConn(): "ok" | "err" | null {
  const [c, setC] = useState<"ok" | "err" | null>(null);
  useEffect(() => {
    connP ??= getMeta()
      .then((m) => (m.connected ? "ok" : "err") as "ok" | "err")
      .catch(() => {
        connP = null;
        return "err" as const;
      });
    connP.then(setC);
  }, []);
  return c;
}
