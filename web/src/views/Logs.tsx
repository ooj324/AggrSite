import { useEffect, useState } from 'react';
import { api } from '../api';
import type { CheckinLog, Event } from '../api';
import { RefreshCw, CheckCircle2, XCircle, Info, AlertCircle, History } from 'lucide-react';
import { format } from 'date-fns';

export default function Logs() {
  const [logs, setLogs] = useState<CheckinLog[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'checkin' | 'events'>('checkin');

  const loadData = async () => {
    setLoading(true);
    try {
      if (activeTab === 'checkin') {
        const res = await api.get('/api/checkin/logs?limit=50');
        setLogs(res.data || []);
      } else {
        const res = await api.get('/api/events?limit=50');
        setEvents(res.data || []);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [activeTab]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Logs & Events</h1>
        <button onClick={loadData} className="btn-secondary flex items-center gap-2">
          <RefreshCw size={18} className={loading ? 'animate-spin' : ''} /> Refresh
        </button>
      </div>

      <div className="flex space-x-1 glass-panel p-1 rounded-xl w-max">
        <button
          onClick={() => setActiveTab('checkin')}
          className={`px-6 py-2 rounded-lg text-sm font-medium transition-all ${
            activeTab === 'checkin' ? 'bg-primary text-white shadow-md' : 'text-textSecondary hover:text-white'
          }`}
        >
          Checkin Logs
        </button>
        <button
          onClick={() => setActiveTab('events')}
          className={`px-6 py-2 rounded-lg text-sm font-medium transition-all ${
            activeTab === 'events' ? 'bg-primary text-white shadow-md' : 'text-textSecondary hover:text-white'
          }`}
        >
          System Events
        </button>
      </div>

      <div className="glass-panel overflow-hidden">
        {loading ? (
          <div className="flex justify-center p-12"><RefreshCw className="animate-spin text-primary" size={32} /></div>
        ) : activeTab === 'checkin' ? (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-white/5 bg-white/5">
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Time</th>
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Account ID</th>
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Status</th>
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Message</th>
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Reward</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/5">
                {logs.map((log, i) => (
                  <tr key={i} className="hover:bg-white/5 transition-colors">
                    <td className="px-6 py-4 text-sm whitespace-nowrap text-textSecondary">
                      {format(new Date(log.created_at), 'MM/dd HH:mm:ss')}
                    </td>
                    <td className="px-6 py-4 text-sm">#{log.account_id}</td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        {log.status === 'success' ? <CheckCircle2 size={16} className="text-success" /> :
                         log.status === 'failed' ? <XCircle size={16} className="text-error" /> :
                         <History size={16} className="text-textSecondary" />}
                        <span className="text-sm capitalize">{log.status}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm text-textSecondary max-w-md truncate" title={log.message}>{log.message}</td>
                    <td className="px-6 py-4 text-sm font-medium text-accent">{log.reward}</td>
                  </tr>
                ))}
                {logs.length === 0 && (
                  <tr><td colSpan={5} className="px-6 py-12 text-center text-textSecondary">No checkin logs found.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-white/5 bg-white/5">
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Time</th>
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Type</th>
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Title</th>
                  <th className="px-6 py-4 text-sm font-medium text-textSecondary">Message</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/5">
                {events.map((ev, i) => (
                  <tr key={i} className="hover:bg-white/5 transition-colors">
                    <td className="px-6 py-4 text-sm whitespace-nowrap text-textSecondary">
                      {format(new Date(ev.created_at), 'MM/dd HH:mm:ss')}
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-xs px-2 py-1 rounded bg-white/10 text-textSecondary uppercase tracking-wider">{ev.type}</span>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        {ev.level === 'error' ? <AlertCircle size={16} className="text-error" /> : <Info size={16} className="text-primary" />}
                        <span className="text-sm font-medium">{ev.title}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm text-textSecondary max-w-md truncate" title={ev.message}>{ev.message}</td>
                  </tr>
                ))}
                {events.length === 0 && (
                  <tr><td colSpan={4} className="px-6 py-12 text-center text-textSecondary">No events found.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
