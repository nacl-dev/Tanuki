import type { Tag } from '@/api/mediaApi'

export function tagExpression(tag: Pick<Tag, 'name' | 'category'>): string {
  if (tag.category === 'general') {
    return tag.name
  }
  return `${tag.category}:${tag.name}`
}

const namespaceMap: Record<string, Tag['category']> = {
  general: 'general',
  tag: 'general',
  tags: 'general',
  artist: 'artist',
  artists: 'artist',
  author: 'artist',
  authors: 'artist',
  creator: 'artist',
  creators: 'artist',
  circle: 'artist',
  circles: 'artist',
  group: 'artist',
  groups: 'artist',
  character: 'character',
  characters: 'character',
  char: 'character',
  parody: 'parody',
  parodies: 'parody',
  copyright: 'parody',
  copyrights: 'parody',
  series: 'parody',
  franchise: 'parody',
  property: 'parody',
  genre: 'genre',
  genres: 'genre',
  male: 'genre',
  female: 'genre',
  mixed: 'genre',
  other: 'genre',
  species: 'genre',
  theme: 'genre',
  themes: 'genre',
  fetish: 'genre',
  fetishes: 'genre',
  category: 'genre',
  categories: 'genre',
  format: 'genre',
  formats: 'genre',
  meta: 'meta',
  title: 'meta',
  page: 'meta',
  rating: 'meta',
  language: 'meta',
  lang: 'meta',
  source: 'meta',
  site: 'meta',
  uploader: 'meta',
  date: 'meta',
}

export function parseTagExpression(raw: string): Pick<Tag, 'name' | 'category'> {
  const normalized = raw.trim().toLowerCase()
  if (!normalized) {
    return { name: '', category: 'general' }
  }

  const separator = normalized.indexOf(':')
  if (separator <= 0) {
    return { name: normalized, category: 'general' }
  }

  const namespace = normalized.slice(0, separator).trim()
  const value = normalized.slice(separator + 1).trim()
  const category = namespaceMap[namespace]
  if (!category || !value) {
    return { name: normalized, category: 'general' }
  }

  return { name: value, category }
}
