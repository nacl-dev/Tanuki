import axios from 'axios'

const rawBasePath = import.meta.env.BASE_URL ?? '/'
const basePath =
  rawBasePath === '/' ? '' : rawBasePath.replace(/\/+$/, '')

function ensureLeadingSlash(path: string) {
  return path.startsWith('/') ? path : `/${path}`
}

export function appPath(path = '/') {
  const normalized = ensureLeadingSlash(path)
  return `${basePath}${normalized}` || '/'
}

export function stripAppBase(path: string) {
  const normalized = ensureLeadingSlash(path || '/')
  if (!basePath || !normalized.startsWith(basePath)) {
    return normalized
  }

  const stripped = normalized.slice(basePath.length)
  return stripped || '/'
}

function currentAppLocation() {
  return `${stripAppBase(window.location.pathname)}${window.location.search}${window.location.hash}`
}

const client = axios.create({
  baseURL: appPath('/api'),
  headers: { 'Content-Type': 'application/json' },
  timeout: 30_000,
  withCredentials: true,
})

// Response interceptor – unwrap the envelope, redirect on 401
client.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      const currentPath = stripAppBase(window.location.pathname)
      const onPublicAuthRoute =
        currentPath.startsWith('/login') ||
        currentPath.startsWith('/register')
      if (!onPublicAuthRoute) {
        const redirect = encodeURIComponent(currentAppLocation())
        window.location.href = appPath(`/login?redirect=${redirect}`)
      }
    }
    const requestId = err.response?.data?.request_id
    const msg = err.response?.data?.error ?? err.message
    const detail = requestId ? `${msg} (Request ID: ${requestId})` : msg
    return Promise.reject(new Error(detail))
  },
)

export default client
