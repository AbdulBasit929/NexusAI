import { parse } from 'yaml'
import accessLog from '@semantic-layer/access_log.yaml?raw'
import anpr from '@semantic-layer/anpr.yaml?raw'
import cdr from '@semantic-layer/cdr.yaml?raw'
import ipdr from '@semantic-layer/ipdr.yaml?raw'
import subscriber from '@semantic-layer/subscriber.yaml?raw'
import tower from '@semantic-layer/tower.yaml?raw'
import transaction from '@semantic-layer/transaction.yaml?raw'

const semanticDocuments = { accessLog, anpr, cdr, ipdr, subscriber, tower, transaction }

function normalized(value) {
  return String(value ?? '').trim().toLocaleLowerCase()
}

function collectAliases(field) {
  return [field.id, field.id?.split('.').pop(), ...(field.source_names || [])]
    .map(normalized)
    .filter(Boolean)
}

export function createSemanticCatalog(documents) {
  const entities = Object.values(documents)
    .map(source => typeof source === 'string' ? parse(source) : source)
    .filter(entity => entity?.record_type)
  const byRecordType = new Map()
  const byId = new Map()
  const valuesByField = new Map()

  for (const entity of entities) {
    const aliases = new Map()
    for (const field of entity.fields || []) {
      byId.set(field.id, { ...field, entity })
      for (const alias of collectAliases(field)) aliases.set(alias, field.id)
      const values = new Map()
      for (const item of field.values || []) values.set(normalized(item.value), item)
      valuesByField.set(field.id, values)
    }
    for (const metric of entity.metrics || []) byId.set(metric.id, { ...metric, entity, metric: true })
    byRecordType.set(normalized(entity.record_type), { ...entity, aliases })
  }

  function entityFor(recordType) {
    const key = normalized(recordType)
    return byRecordType.get(key)
      || [...byRecordType.values()].find(entity => normalized(entity.family) === key)
      || null
  }

  function fieldFor(key, recordType) {
    if (!key) return null
    if (byId.has(key)) return byId.get(key)
    const entity = entityFor(recordType)
    const id = entity?.aliases.get(normalized(key))
    if (id) return byId.get(id)
    const matches = [...byId.values()].filter(item => !item.metric && collectAliases(item).includes(normalized(key)))
    return matches.length === 1 ? matches[0] : null
  }

  function measureFor(columnKey, plan, recordType) {
    const measure = (plan?.measures || []).find(item => normalized(item.measure_id) === normalized(columnKey))
    if (!measure) return null
    const entity = entityFor(recordType)
    if (normalized(measure.op) === 'count' && (!measure.field_id || (plan?.group_fields || []).includes(measure.field_id))) {
      if (entity?.default_measure && byId.has(entity.default_measure)) return byId.get(entity.default_measure)
    }
    const declaredMetric = (entity?.metrics || []).find(metric => (
      normalized(metric.field_id) === normalized(measure.field_id)
      && normalized(metric.aggregate) === normalized(measure.op)
    ))
    if (declaredMetric && byId.has(declaredMetric.id)) return byId.get(declaredMetric.id)
    if (measure.field_id && byId.has(measure.field_id)) return byId.get(measure.field_id)
    if (entity?.default_measure && byId.has(entity.default_measure)) return byId.get(entity.default_measure)
    return null
  }

  function displayForColumn(key, recordType, plan) {
    const item = /^m\d+$/i.test(String(key)) ? measureFor(key, plan, recordType) : fieldFor(key, recordType)
    return item
      ? { id: item.id, label: item.display_name, description: item.description || '', fallback: false }
      : { id: '', label: String(key), description: '', fallback: true }
  }

  function displayForValue(fieldId, value) {
    const item = valuesByField.get(fieldId)?.get(normalized(value))
    return item?.display_name || value
  }

  return { entities, byId, entityFor, fieldFor, displayForColumn, displayForValue }
}

export const semanticCatalog = createSemanticCatalog(semanticDocuments)

export function curatedFamilyLabel(recordType) {
  return semanticCatalog.entityFor(recordType)?.display_name || String(recordType || '')
}
