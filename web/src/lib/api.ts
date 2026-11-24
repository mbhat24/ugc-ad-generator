import "server-only";
import { redirect } from "next/navigation";
import { accessToken } from "./session";
import type { Me, Project } from "./types";

export const API_URL = (process.env.API_URL ?? "http://localhost:8080").replace(/\/$/, "");

export async function apiFetch(path: string, init: RequestInit = {}) {
  const token = await accessToken();
  const headers = new Headers(init.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  return fetch(`${API_URL}${path}`, { ...init, headers, cache: "no-store" });
}

async function json<T>(path: string): Promise<T> {
  const r = await apiFetch(path);
  if (r.status === 401) redirect("/login");
  if (!r.ok) throw new Error(`API ${path} failed: ${r.status}`);
  return r.json();
}

export const getMe = () => json<Me>("/v1/me");
export const listProjects = () => json<Project[]>("/v1/projects");

export async function getProject(id: string): Promise<Project | null> {
  const r = await apiFetch(`/v1/projects/${encodeURIComponent(id)}`);
  if (r.status === 401) redirect("/login");
  if (r.status === 404) return null;
  if (!r.ok) throw new Error(`API project failed: ${r.status}`);
  return r.json();
}
