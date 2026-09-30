import { LanguageText } from '../components/AnalystComponents.jsx'
import { AppShell } from '../components/CaseShell.jsx'
import { DesignSystemGallery } from '../components/DesignSystemGallery.jsx'
import { PageHeader } from '../components/PageHeader.jsx'

const proofStrings = [
  { id: 'mixed-urdu', value: 'کال 03001234567 at 1035' },
  { id: 'urdu-long', value: 'مجھے معلوم نہیں آیا آپ نے محسوس کیا یا نہیں اس ملک میں سینٹر لمڈیکہ سے' },
  { id: 'roman-urdu', value: 'kal 03001234567 at 1035' },
  { id: 'urdu-second', value: 'داری کام بہنی کھین چک ساتھ ہے' },
]

const typeSteps = ['caption', 'label', 'body-sm', 'body', 'title-sm', 'title', 'display']

export default function TypeProofPage() {
  return (
    <AppShell>
      <main id="workspace-main" className="type-proof" tabIndex={-1}>
        <PageHeader eyebrow="Design verification" title="Visual system gallery" description="Mineral Signal foundations, interaction states and mixed-script typography in one review surface." breadcrumbs={[{ label: 'Workspace', to: '/' }, { label: 'Design verification' }]} />
        <DesignSystemGallery />
        <header className="type-proof__heading"><span className="eyebrow">Typography</span><h2>Mixed-script type proof</h2><p>Noto Sans, Noto Sans Arabic and Noto Sans Mono across the complete analyst scale.</p></header>
        {typeSteps.map(step => (
          <section className="type-proof__step" key={step} data-proof-step={step}>
            <h2>{step}</h2>
            <div className="type-proof__samples">
              {proofStrings.map(sample => (
                <LanguageText
                  as="p"
                  key={sample.id}
                  className={`type-step type-step--${step}`}
                  data-proof-script={sample.id}
                >
                  {sample.value}
                </LanguageText>
              ))}
            </div>
          </section>
        ))}
        <section className="type-proof__baseline" aria-labelledby="baseline-title">
          <h2 id="baseline-title">Shared baseline</h2>
          <div><LanguageText data-baseline="urdu">کال 03001234567 at 1035</LanguageText><LanguageText data-baseline="roman">kal 03001234567 at 1035</LanguageText></div>
        </section>
      </main>
    </AppShell>
  )
}
