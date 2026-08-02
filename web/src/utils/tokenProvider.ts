export type TokenType = 'auth' | 'refresh'

export interface TokenStore {
    getToken(type?: TokenType): string | undefined
    setToken(token: string, type: TokenType): void
    removeToken(): void
}

export interface TokenRefreshContext {
    provider: TokenProvider
    lang: string
    refreshToken: string | undefined
    token: string | undefined
}

export interface TokenProvider {
    domain: string
    header: string
    store: () => TokenStore
    loginRoute: string | (() => void)
    refreshType?: string
    refresher?: (context: TokenRefreshContext) => Promise<string>
    useAnotherToken?: boolean
}

const tokenProviders = new Map<string, TokenProvider>()

export function registerTokenProvider(provider: TokenProvider) {
    tokenProviders.set(provider.domain, provider)
}

export function getTokenProvider(domain: string) {
    const provider = tokenProviders.get(domain)
    if (!provider) {
        throw new Error(`Token provider is not registered: ${domain}`)
    }
    return provider
}

export function getTokenProviderByRefreshType(refreshType: string) {
    return Array.from(tokenProviders.values()).find((provider) => provider.refreshType == refreshType)
}

export function getTokenProviders() {
    return Array.from(tokenProviders.values())
}
