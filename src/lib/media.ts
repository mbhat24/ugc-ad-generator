import type { Project } from "./db";
import { signedVideoUrl } from "./storage";

export async function signProject(p: Project): Promise<Project> {
  const ads = await Promise.all(p.ads.map(async ({ videoKey, ...a }) => (videoKey ? { ...a, videoUrl: await signedVideoUrl(videoKey) } : a)));
  return { ...p, ads };
}
