// @ts-nocheck
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'

// @ts-ignore - simple route definition retained
export const Route = createFileRoute('/nodes')({
  beforeLoad: () => {
    const token = useAuthStore.getState().auth.accessToken
    if (!token) {
      throw redirect({ to: '/login' })
    }

    throw redirect({ to: '/parking', search: { tab: 'spaces' } })
  },
  component: () => null,
})
