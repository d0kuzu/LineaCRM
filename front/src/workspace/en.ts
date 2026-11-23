import type { CountMessage } from '../plurals';

export const workspaceEn = {
  general: {
    company: 'Company', owner: 'Owner', amount: 'Amount', contact: 'Contact',
    forecast: 'Forecast', target: 'Target', nextStep: 'Next step', stage: 'Stage',
    all: 'All', openDeal: 'Open deal', openBrief: 'Open meeting brief',
    noResults: 'No matching records.', searchContacts: 'Search contacts',
    close: 'Close', date: 'Date', time: 'Time', companyDirectory: 'Company directory',
  },
  customers: {
    description: 'A shared customer directory for the sales team.',
    contacts: 'Contacts', companies: 'Companies', contactDirectory: 'Contact directory',
  },
  sales: {
    title: 'Deal pipeline', description: 'Main pipeline · 12 open deals',
    board: 'Board', list: 'List', allDeals: 'All deals', risk: 'At risk', deal: 'Deal',
    stages: { new: 'New', discovery: 'Discovery', negotiation: 'Negotiation', contract: 'Contract' },
    opportunities: {
      integration: 'Email integration', training: 'Team training', office: 'Office support',
      workstations: 'Workstations', branch: 'CRM for the branch office',
      fleet: 'Fleet upgrade', support: 'Support line',
    },
    activities: {
      qualification: 'Today · Qualification', intro: 'Tomorrow · Intro call', call: '{date} · Call',
      approval: '{date} · Approval', demo: '{date} · Demo', meeting: '{date} · Meeting',
      todayDemo: 'Today · Demo at {time}', inactive: 'No activity for 7 days',
      todayMeeting: 'Today · Meeting', contract: '{date} · Contract', overdue: 'Task overdue',
    },
  },
  tasks: {
    title: 'My tasks', description: 'Sales rep’s work plan', today: 'Today', completed: 'Completed',
    dueToday: 'Due today', completedTitle: 'Completed tasks', firstFour: 'First 4 of 8 tasks · In priority order',
    remaining: { one: '{count} priority task remaining', other: '{count} priority tasks remaining' } as CountMessage,
    empty: 'No tasks in this view.', nextMeeting: 'Next meeting', demo: 'Product demo',
    aim: 'Agree on who will join the pilot and when it will start.',
    names: ['Prepare for the demo', 'Run the product demo', 'Confirm the approval deadline', 'Call the new customer'],
    contacts: ['Orbita · CRM implementation', 'Orbita · Aliya Kim', 'Terra Logistics · Fleet upgrade', 'Qala Studio · Workstations'],
  },
  calendar: {
    title: 'Calendar', weekOf: 'Week of', week: 'Week', day: 'Day', today: 'Today',
    noEvents: 'No meetings scheduled.', events: 'Scheduled events', selectedDay: 'Choose a day',
    names: { sync: 'Team sync', meeting: 'Meeting', demo: 'Demo', terms: 'Terms', procurement: 'Procurement', call: 'Call', pilot: 'Pilot', proposal: 'Proposal' },
    team: 'Sales team',
  },
  inbox: {
    title: 'Inbox', description: 'Email, calls and messages in one inbox.', all: 'All activity',
    email: 'Email', calls: 'Calls', messaging: 'Messaging', channels: 'Channels',
    incoming: 'Incoming', contact: 'Contact', dealContext: 'Deal context', assistant: 'Assistant',
    role: 'Operations director · Orbita', greeting: 'Hi Maria,',
    proposal: 'Thanks for the call. I’m looking forward to the final CRM implementation proposal.',
    timeline: 'Please include the timeline and steps for connecting our phone system.',
    note: 'On the Nov 4 call, you agreed to send the proposal by Friday.',
    subjects: ['Re: CRM proposal', 'Missed call', 'Calendar setup', 'Updated contract', 'Demo recording', 'Invitation response'],
  },
  analytics: {
    title: 'Sales analytics', description: 'Sales team / November target and forecast',
    summary: 'Target and forecast', pipeline: 'Pipeline', team: 'Team', reps: 'Sales reps', rep: 'Sales rep',
    actual: 'Actual', atRisk: 'At risk', metrics: ['Monthly target', 'Actual revenue', 'Revenue forecast', 'Gap to target'],
    notes: ['Team target for November', '23% of monthly target', 'At the current sales pace', '15% of monthly target'],
    byWeek: 'By week', revenue: 'Revenue trend', week: 'Week', riskTitle: 'What’s putting the target at risk?',
    assistant: 'Assistant / Sales team', count: '3 deals need attention.',
    cause: 'Slipping deadlines and missing next steps are lowering the forecast.',
    sources: 'Sources', sourceNote: '3 deal records · 5 events', review: 'Review risks', singleDeal: '1 deal',
    risks: ['No task to send the proposal.', 'The customer hasn’t replied in seven days.', 'Approval has been postponed twice.'],
  },
};
