# AI Generate: Memory and System Prompt

The **AI generate** automation step can remember previous conversations. When memory is enabled, the earlier inputs and AI answers with the same **session key** are sent to the model as chat history, so a Telegram bot or a support assistant can follow up on what was said before.

## Enable it

1. Open the automation, click the **AI generate** node and turn on **Memory > Remember conversations**.
2. **Session key** - a template that identifies the conversation, e.g.
   - `{{trigger.telegram.chat.id}}` - one conversation per Telegram chat
   - `{{input.userId}}` - one per user for manual/called automations
   - `order-{{record.id}}` - one per record
   Leave it empty to share a single memory (`default`) for every run.
3. **Messages to remember** - how many of the latest messages are kept and sent (default 20, max 100; one question + one answer = 2 messages).
4. Save the automation.

Step JSON:

```json
{
  "type": "ai.generate",
  "input": "{{trigger.telegram.message.text}}",
  "memory": { "enabled": true, "key": "{{trigger.telegram.chat.id}}", "maxMessages": 20 }
}
```

The step output gets a `memory` object with the resolved `key` and the number of `history` messages sent:

```json
{ "output": { "text": "..." }, "memory": { "key": "123456", "history": 4 } }
```

## How it works

- Memory belongs to the automation; different automations never share memory. Several AI generate steps of the same automation share it when their session keys are equal - add a prefix (e.g. `support-{{...}}`) to keep them apart.
- Only the rendered **input** and the **final answer** are stored (MCP tool calls are not). Structured (output schema) answers are stored as JSON.
- After each run only the latest *Messages to remember* messages of the session are kept.
- Dry runs don't read or write memory.
- Works with all AI providers (OpenAI/OpenAI-compatible, Anthropic, Gemini) and together with [MCP tools](mcp-client.md).
- Deleting the automation deletes its memory.

## System prompt

The **System prompt** field of the AI generate node sets the AI's role, tone and rules (e.g. *"You are a support assistant for Acme. Answer in Vietnamese, briefly. Never promise refunds."*). It supports templates (`{{input.*}}`, `{{record.*}}`, `{{zalo.*}}`...), is sent with every request - including each round of the MCP tool loop - and is **not** stored in the memory, so changing it applies to existing conversations right away.

It's sent as the provider's native system instruction: a leading `system` message (OpenAI-compatible), the `system` field (Anthropic) or `systemInstruction` (Gemini).

```json
{ "type": "ai.generate", "systemPrompt": "You are a support assistant for {{automation.name}}.", "input": "{{zalo.text}}" }
```

## Manage memory

Click **Manage memory** in the node to browse the sessions and their messages, clear a single session or clear everything.

API (superuser only):

| Method   | Route                                   | Description                                                |
| -------- | --------------------------------------- | ---------------------------------------------------------- |
| `GET`    | `/api/automations/{id}/ai-memory`       | `{ items: [{ key, messages, updated }] }`                  |
| `GET`    | `/api/automations/{id}/ai-memory?key=k` | `{ key, messages: [{ role, content }] }` (oldest first)    |
| `DELETE` | `/api/automations/{id}/ai-memory`       | Clears all sessions; add `?key=k` to clear one (`204`)     |

## Code Locations

- `core/automation_ai_memory.go`, `migrations/1786000002_ai_memory.go` - `_aiMemory` table and helpers
- `core/automation_ai.go` - loading/saving the memory around the AI call; providers receive it as `AutomationAIRequest.History`
- `apis/automation_ai_memory.go` - memory API
- `ui/src/settings/automations/stepEditor.js` (`aiMemoryEditor`), `ui/src/settings/automations/aiMemoryModal.js` - UI
