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
    <div className="animate-fade-in">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
        <h2 className="greeting">日志与事件</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button onClick={loadData} className="btn btn-secondary">
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} style={{ marginRight: 6 }} /> 
            刷新
          </button>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 4, background: 'var(--color-bg)', padding: 4, borderRadius: 'var(--radius-md)', width: 'max-content', marginBottom: 24, border: '1px solid var(--color-border)' }}>
        <button
          onClick={() => setActiveTab('checkin')}
          className="btn"
          style={{
            background: activeTab === 'checkin' ? 'var(--color-primary)' : 'transparent',
            color: activeTab === 'checkin' ? 'var(--color-white)' : 'var(--color-text-secondary)',
            border: 'none',
            boxShadow: activeTab === 'checkin' ? '0 2px 4px rgba(0,0,0,0.1)' : 'none',
          }}
        >
          签到日志
        </button>
        <button
          onClick={() => setActiveTab('events')}
          className="btn"
          style={{
            background: activeTab === 'events' ? 'var(--color-primary)' : 'transparent',
            color: activeTab === 'events' ? 'var(--color-white)' : 'var(--color-text-secondary)',
            border: 'none',
            boxShadow: activeTab === 'events' ? '0 2px 4px rgba(0,0,0,0.1)' : 'none',
          }}
        >
          系统事件
        </button>
      </div>

      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        {loading ? (
          <div className="flex justify-center p-12">
            <span className="spinner spinner-lg text-primary" />
          </div>
        ) : activeTab === 'checkin' ? (
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>账户 ID</th>
                  <th>状态</th>
                  <th>消息</th>
                  <th>奖励</th>
                </tr>
              </thead>
              <tbody>
                {logs.map((log, i) => (
                  <tr key={i}>
                    <td style={{ color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                      {format(new Date(log.created_at), 'MM/dd HH:mm:ss')}
                    </td>
                    <td>#{log.account_id}</td>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        {log.status === 'success' ? <CheckCircle2 size={16} color="var(--color-success)" /> :
                         log.status === 'failed' ? <XCircle size={16} color="var(--color-danger)" /> :
                         <History size={16} color="var(--color-text-secondary)" />}
                        <span style={{ fontSize: 13 }}>
                          {log.status === 'success' ? '成功' : log.status === 'failed' ? '失败' : '待处理'}
                        </span>
                      </div>
                    </td>
                    <td style={{ maxWidth: 300, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', color: 'var(--color-text-secondary)' }} title={log.message}>{log.message}</td>
                    <td style={{ fontWeight: 500, color: 'var(--color-primary)' }}>{log.reward}</td>
                  </tr>
                ))}
                {logs.length === 0 && (
                  <tr><td colSpan={5} style={{ textAlign: 'center', padding: 48, color: 'var(--color-text-secondary)' }}>未找到签到日志。</td></tr>
                )}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>类型</th>
                  <th>标题</th>
                  <th>消息</th>
                </tr>
              </thead>
              <tbody>
                {events.map((ev, i) => (
                  <tr key={i}>
                    <td style={{ color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                      {format(new Date(ev.created_at), 'MM/dd HH:mm:ss')}
                    </td>
                    <td>
                      <span className="badge">{ev.type}</span>
                    </td>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        {ev.level === 'error' ? <AlertCircle size={16} color="var(--color-danger)" /> : <Info size={16} color="var(--color-primary)" />}
                        <span style={{ fontWeight: 500 }}>{ev.title}</span>
                      </div>
                    </td>
                    <td style={{ maxWidth: 400, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', color: 'var(--color-text-secondary)' }} title={ev.message}>{ev.message}</td>
                  </tr>
                ))}
                {events.length === 0 && (
                  <tr><td colSpan={4} style={{ textAlign: 'center', padding: 48, color: 'var(--color-text-secondary)' }}>未找到事件。</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
