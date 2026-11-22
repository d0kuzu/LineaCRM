export const topicIds = ['prep', 'inactive', 'atlas'] as const;
export type Topic = typeof topicIds[number];
export type Answer = Topic | 'mayak' | 'help';
type Exchange = { question: string; answer: Answer };
export type Conversation = { id: number; topic?: Topic; exchanges: Exchange[] };
export type ChatSession = { current: number; threads: Conversation[] };

const storageKey = 'linea.assistant.session.v1';
const answers: ReadonlySet<string> = new Set([...topicIds, 'mayak', 'help']);
let memory: ChatSession | undefined;

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isConversation(value: unknown): value is Conversation {
  if (!isObject(value) || !Number.isSafeInteger(value.id) || !Array.isArray(value.exchanges)) return false;
  if (value.topic !== undefined && !topicIds.some(topic => topic === value.topic)) return false;
  return value.exchanges.every(entry => isObject(entry) && typeof entry.question === 'string'
    && typeof entry.answer === 'string' && answers.has(entry.answer));
}

function isSession(value: unknown): value is ChatSession {
  if (!isObject(value) || !Array.isArray(value.threads) || !value.threads.every(isConversation)) return false;
  const ids = new Set(value.threads.map(thread => thread.id));
  return ids.size === value.threads.length && typeof value.current === 'number' && ids.has(value.current);
}

export function loadChatSession(): ChatSession {
  if (memory) return memory;
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null');
    if (isSession(value)) return value;
  } catch { /* The tab can still use in-memory conversations. */ }
  return { current: 0, threads: topicIds.map((topic, id) => ({ topic, id, exchanges: [] })) };
}

export function saveChatSession(session: ChatSession) {
  memory = session;
  try {
    sessionStorage.setItem(storageKey, JSON.stringify(session));
  } catch { /* Keep the session available across routes if storage is blocked or full. */ }
}
