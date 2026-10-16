import { CARRIERS, carrierIconUrl } from '../lib/carriers.js'

// The carrier of a SIM or of a number, as a badge and, where there is room, its name. The name is always in the accessible text: colour and a letter never carry the meaning alone.
// The badge is an original placeholder mark, not the carrier's logo (a logo needs a licence-ledger entry first); put an official file in public/carriers/<id>.svg to use it.
export function CarrierBadge({ carrier, size = 18, showName = false, how = '', className = '' }) {
  const c = CARRIERS[carrier] || CARRIERS.other
  return (
    <span className={`carrier-badge${className ? ` ${className}` : ''}`} title={how ? `${c.name}: found from ${how}` : c.name}>
      <img src={carrierIconUrl(c.id, size * 2)} alt="" width={size} height={size} decoding="async" />
      {showName ? <span className="carrier-badge__name">{c.name}</span> : <span className="visually-hidden">{c.name}</span>}
    </span>
  )
}
