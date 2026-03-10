import { defineStore } from 'pinia'
import { ref } from 'vue'
import { pluginApi, type Plugin } from '@/api/pluginApi'

export const usePluginStore = defineStore('plugin', () => {
    const plugins = ref<Plugin[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)

    async function fetchPlugins() {
        loading.value = true
        error.value = null
        try {
            const res = await pluginApi.list()
            plugins.value = res.data ?? []
        } catch (e: any) {
            error.value = e.message
        } finally {
            loading.value = false
        }
    }

    async function scanPlugins() {
        loading.value = true
        error.value = null
        try {
            const res = await pluginApi.scan()
            plugins.value = res.data ?? []
        } catch (e: any) {
            error.value = e.message
        } finally {
            loading.value = false
        }
    }

    async function togglePlugin(id: string, enabled: boolean) {
        await pluginApi.toggle(id, enabled)
        const idx = plugins.value.findIndex((p) => p.id === id)
        if (idx !== -1) plugins.value[idx].enabled = enabled
    }

    async function removePlugin(id: string) {
        await pluginApi.remove(id)
        plugins.value = plugins.value.filter((p) => p.id !== id)
    }

    return {
        plugins,
        loading,
        error,
        fetchPlugins,
        scanPlugins,
        togglePlugin,
        removePlugin,
    }
})
