import Legal from "@/components/Legal";

export const metadata = { title: "Acceptable Use Policy - AdReel" };

export default function Page() {
  return (
    <Legal title="Acceptable Use Policy">
      <p>You may not use AdReel to create content for:</p>
      <ul>
        <li>Weapons, ammunition and weapon accessories</li>
        <li>Adult or sexually explicit products and services</li>
        <li>Illegal drugs, cannabis and related paraphernalia; tobacco, vaping and nicotine products</li>
        <li>Gambling and betting</li>
        <li>Counterfeit or infringing goods</li>
        <li>Deceptive financial offers, get-rich-quick schemes and unregulated investment advice</li>
        <li>Unsubstantiated health claims, miracle cures or medical advice</li>
        <li>Harassment, hate, impersonation of real people or any unlawful purpose</li>
      </ul>
      <p>Do not fabricate customer testimonials or present AI-generated people as real customers. Label AI-generated content where platforms or laws require it. Automated checks may block submissions; we may suspend accounts that violate this policy. Report abuse to [ABUSE EMAIL].</p>
    </Legal>
  );
}
