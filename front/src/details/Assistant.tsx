import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useLocale } from '../i18n';
import { deals } from '../overview/data';
import { money } from '../workspace/format';
import { PageTitle } from '../workspace/shared';
import assistantIcon from './assets/assistant.svg';
import sendIcon from './assets/send.svg';
import { fill } from './format';
import { loadChatSession, saveChatSession, topicIds, type Answer, type Conversation } from './conversations';

const topicAccounts = { prep: 'orbita', inactive: 'tochka', atlas: 'atlas' } as const;

function Reply({ answer }: { answer: Answer }) {
  const { messages: { details: { assistant: copy }, overview }, locale } = useLocale();
  const author = <div className="detail-bot-name"><img src={assistantIcon} alt="" />{copy.author}</div>;
  if (answer === 'help') return <div className="detail-reply">{author}<p>{copy.unsupported}</p>
    <a className="detail-source" href="#/search">{copy.search}</a></div>;
  if (answer === 'mayak') return <div className="detail-reply">{author}<h3>{overview.accounts.mayak.deal}</h3>
    <p>{overview.accounts.mayak.alert}</p><a className="detail-source" href="#/deal/mayak">{copy.dealRecord}</a></div>;
  const topic = copy.topics[answer];
  const id = topicAccounts[answer];
  return <div className="detail-reply">{author}<h3>{topic.answerTitle}</h3>
    <p>{fill(topic.answer, { amount: money(deals[0].amount, locale, true), reps: (12).toLocaleString(locale) })}</p>
    {topic.emphasis && <div className="detail-goal"><small>{copy.goal}</small><strong>{topic.emphasis}</strong><p>{topic.followUp}</p></div>}
    <h4>{copy.sources}</h4><div className="detail-sources">
      <a className="detail-source" href={'#/meeting/' + id}>{copy.meetingBrief}</a>
      <a className="detail-source" href={'#/deal/' + id}>{copy.dealRecord}</a>
    </div>
  </div>;
}

export function Assistant() {
  const { messages: { details: { assistant: copy }, overview }, locale } = useLocale();
  const [session, setSession] = useState(loadChatSession);
  const { current, threads } = session;
  useEffect(() => { saveChatSession(session); }, [session]);
  const [input, setInput] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);
  const thread = threads.find(item => item.id === current)!;
  const titleFor = (item: Conversation) => item.topic ? copy.topics[item.topic].title : item.exchanges[0]?.question || copy.untitled;
  const exchanges = thread.topic
    ? [{ question: copy.topics[thread.topic].prompt, answer: thread.topic }, ...thread.exchanges]
    : thread.exchanges;
  function createChat() {
    const existing = threads.find(item => !item.topic && item.exchanges.length === 0);
    const id = existing?.id ?? Math.max(...threads.map(item => item.id)) + 1;
    setSession({ current: id, threads: existing ? threads : [{ id, exchanges: [] }, ...threads] });
    setInput('');
    inputRef.current?.focus();
  }
  function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const question = input.trim();
    if (!question) return;
    const query = question.toLocaleLowerCase(locale);
    const account = deals.find(deal => [overview.accounts[deal.id].company, overview.accounts[deal.id].deal]
      .some(label => query.includes(label.toLocaleLowerCase(locale))))?.id;
    const topic = account ? topicIds.find(id => topicAccounts[id] === account) : topicIds.find(
      id => copy.keywords[id].some(word => query.includes(word.toLocaleLowerCase(locale))));
    const answer: Answer = account === 'mayak' ? 'mayak' : topic ?? 'help';
    setSession(previous => ({ ...previous, threads: previous.threads.map(item => item.id === current
      ? { ...item, exchanges: [...item.exchanges, { question, answer }] } : item) }));
    setInput('');
    inputRef.current?.focus();
  }

  return <>
    <PageTitle title={copy.title} description={copy.subtitle} />
    <div className="detail-assistant-grid">
      <aside className="detail-card detail-conversations"><h2>{copy.conversations}</h2>
        <button type="button" className="detail-new-chat" onClick={createChat}>{copy.newChat}</button>
        <h3>{copy.today}</h3>
        {threads.map(item => <button type="button" key={item.id} aria-pressed={item.id === current}
          className="detail-thread" onClick={() => { setSession(previous => ({ ...previous, current: item.id })); setInput(''); }}>
          <strong>{titleFor(item)}</strong>{item.topic && <small>{copy.topics[item.topic].preview}</small>}
        </button>)}
        <p className="detail-demo-note">{copy.demo}</p>
      </aside>
      <section className="detail-card detail-chat">
        <header><h2>{titleFor(thread)}</h2><p className="detail-muted">{fill(copy.user, { owner: overview.owner })}</p></header>
        <div className="detail-transcript" aria-live="polite" aria-relevant="additions">
          {exchanges.map((exchange, index) => <div key={thread.id + '-' + index}>
            <p className="detail-prompt">{exchange.question}</p><Reply answer={exchange.answer} />
          </div>)}
          {exchanges.length === 0 && <p>{copy.welcome}</p>}
        </div>
        <form className="detail-composer" onSubmit={send}>
          <input ref={inputRef} aria-label={copy.placeholder} placeholder={copy.placeholder} value={input}
            onChange={event => setInput(event.target.value)} maxLength={500} />
          <button type="submit" aria-label={copy.send} disabled={!input.trim()}><img src={sendIcon} alt="" /></button>
        </form>
      </section>
    </div>
  </>;
}
