import Legal from "@/components/Legal";

export const metadata = { title: "Privacy Policy - AdReel" };

export default function Page() {
  return (
    <Legal title="Privacy Policy">
      <p>Last updated: [DATE]. Controller: [COMPANY LEGAL NAME], [ADDRESS]. Contact: [PRIVACY EMAIL].</p>
      <h2>Data we collect</h2>
      <ul>
        <li>Account data: email address and authentication data (handled by Supabase).</li>
        <li>Content you submit: product URLs and the product information we retrieve from them.</li>
        <li>Generated content: scripts, captions and videos.</li>
        <li>Billing data: handled by Stripe; we store a customer ID and plan, not card numbers.</li>
        <li>Technical data: IP address and logs for security, abuse prevention and rate limiting.</li>
      </ul>
      <h2>How we use it</h2>
      <p>To provide the service, process payments, prevent abuse, provide support and comply with law. Legal bases (GDPR): contract, legitimate interests and legal obligation.</p>
      <h2>Who receives it (subprocessors)</h2>
      <ul>
        <li>Supabase (authentication and database)</li>
        <li>Stripe (payments)</li>
        <li>Higgsfield (video generation; receives product text and images and the generated script)</li>
        <li>OpenAI (script writing and moderation, if enabled)</li>
        <li>[STORAGE/CDN PROVIDER] and [HOSTING PROVIDER]</li>
      </ul>
      <h2>Retention</h2>
      <p>We keep account data while your account is active. Generated videos are kept for [RETENTION PERIOD]. Financial records are kept as required by law. Provider-side copies of generations are deleted after we store our copy.</p>
      <h2>Your rights</h2>
      <p>You can export your data from the dashboard (<code>/api/account/export</code>) and delete your account (<code>DELETE /api/account</code>), which cancels subscriptions and removes your content. You may also contact us to exercise rights of access, rectification, objection and complaint to your data protection authority.</p>
      <h2>International transfers and security</h2>
      <p>Data may be processed outside your country with appropriate safeguards. We use encryption in transit, access controls and rate limiting.</p>
    </Legal>
  );
}
