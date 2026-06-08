import { redirect } from "next/navigation";

// Entrada da aplicação → encaminha para o login (protótipo de front).
export default function Home() {
  redirect("/login");
}
