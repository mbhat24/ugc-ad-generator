import Legal from "@/components/Legal";

export const metadata = { title: "Terms of Service - AdReel" };

export default function Page() {
  return (
    <Legal title="Terms of Service">
      <p>Last updated: [DATE]. These terms govern your use of AdReel, operated by [COMPANY LEGAL NAME] ([ADDRESS], [COUNTRY]).</p>
      <h2>1. The service</h2>
      <p>AdReel generates scripts, captions and AI-generated videos from product pages you submit. Outputs are produced by automated systems and may contain errors. You are responsible for reviewing outputs before publishing.</p>
      <h2>2. Accounts</h2>
      <p>You must provide accurate information, keep your credentials secure and be at least 18 years old. You are responsible for activity under your account.</p>
      <h2>3. Credits, plans and billing</h2>
      <ul>
        <li>Each generated video uses one credit. Credits from a failed generation are returned automatically.</li>
        <li>Paid plans renew monthly until cancelled. You can cancel anytime from the billing portal; access continues until the end of the paid period.</li>
        <li>Payments are processed by Stripe. Taxes may apply.</li>
        <li>Unless required by law, fees are non-refundable once credits have been used. [REFUND POLICY]</li>
      </ul>
      <h2>4. Your content and outputs</h2>
      <p>You confirm you have the right to promote the products you submit. Subject to these terms and applicable law, you may use generated outputs for your business, including advertising. We may use anonymised, aggregated usage data to operate and improve the service.</p>
      <h2>5. AI-generated media</h2>
      <p>Videos feature synthetic people. You must comply with platform and legal requirements to label AI-generated content, and must not present synthetic people as real customer testimonials where that would be misleading.</p>
      <h2>6. Acceptable use</h2>
      <p>You must follow the Acceptable Use Policy. We may refuse or remove content and suspend accounts that violate it.</p>
      <h2>7. Third-party services</h2>
      <p>We use providers including Supabase, Stripe, our video generation provider and cloud hosting. Their availability may affect the service.</p>
      <h2>8. Availability and changes</h2>
      <p>We aim for high availability but do not guarantee uninterrupted service. We may modify the service or these terms with notice.</p>
      <h2>9. Liability</h2>
      <p>To the extent permitted by law, the service is provided &quot;as is&quot; and our total liability is limited to the fees you paid in the 12 months before the claim. [JURISDICTION-SPECIFIC TERMS]</p>
      <h2>10. Contact</h2>
      <p>[SUPPORT EMAIL]</p>
    </Legal>
  );
}
