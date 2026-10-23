import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api } from '#/lib/api'
import { EmailTab } from './email-tab'
import type { SystemSettings } from './types'

vi.mock('#/lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('#/lib/api')>()),
  api: { post: vi.fn() },
}))

const SEND_TEST_EMAIL_PATH = '/api/admin/settings/send-test-email'
const RECIPIENT = 'ops@example.com'

function renderEmailTab() {
  render(<EmailTab settings={{} as SystemSettings} setSettings={vi.fn()} />)
}

async function sendTestEmailTo(recipient: string) {
  const user = userEvent.setup()
  await user.type(screen.getByRole('textbox', { name: 'Test email recipient' }), recipient)
  await user.click(screen.getByRole('button', { name: 'Send Test' }))
}

afterEach(() => {
  cleanup()
  vi.mocked(api.post).mockReset()
})

describe('EmailTab', () => {
  describe('send test email', () => {
    it('should post the recipient and show the server message on success', async () => {
      vi.mocked(api.post).mockResolvedValue({ status: 'ok', message: `Test email sent to ${RECIPIENT}` })
      renderEmailTab()

      await sendTestEmailTo(RECIPIENT)

      expect(api.post).toHaveBeenCalledWith(SEND_TEST_EMAIL_PATH, { to: RECIPIENT })
      expect(await screen.findByRole('status')).toHaveProperty('textContent', `Test email sent to ${RECIPIENT}`)
    })

    it('should show the server error message when sending fails', async () => {
      const serverMessage = 'smtp send: smtp dial: connection refused'
      vi.mocked(api.post).mockRejectedValue(new ApiError(400, '', { error: serverMessage }, serverMessage))
      renderEmailTab()

      await sendTestEmailTo(RECIPIENT)

      expect(await screen.findByRole('alert')).toHaveProperty('textContent', serverMessage)
    })

    it('should explain that the test uses the saved mail settings', () => {
      renderEmailTab()

      expect(screen.getByText(/saved mail settings/i)).toBeTruthy()
    })
  })
})
