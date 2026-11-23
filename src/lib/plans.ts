export type PlanId = "free" | "starter" | "growth" | "scale";

export const PLANS: Record<PlanId, { name: string; price: number; videos: number; blurb: string }> = {
  free: { name: "Free trial", price: 0, videos: 1, blurb: "Try one ad on us" },
  starter: { name: "Starter", price: 39, videos: 12, blurb: "For new stores testing creatives" },
  growth: { name: "Growth", price: 99, videos: 40, blurb: "For stores running ads weekly" },
  scale: { name: "Scale", price: 249, videos: 120, blurb: "For agencies and high-volume brands" },
};
