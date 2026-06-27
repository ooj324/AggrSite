import { useEffect, useState } from 'react';
import { api } from '../api';
import type { CheckinLog, Event } from '../api';
import { RefreshCw } from 'lucide-react';
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
      <div className="page-header">
        <h2 className="page-title">日志与事件</h2>
        <button onClick={loadData} className="btn btn-soft-primary">
          {loading ? <span className="spinner spinner-sm" style={{ marginRight: 6 }} /> : <RefreshCw size={16} style={{ marginRight: 6 }} />}
          刷新
        </button>
      </div>

      <div className="pill-tabs" style={{ marginBottom: 16 }}>
        <button
          onClick={() => setActiveTab('checkin')}
          className={`pill-tab ${activeTab === 'checkin' ? 'active' : ''}`}
        >
          签到日志 <span style={{ fontVariantNumeric: "tabular-nums", opacity: 0.7 }}>{logs.length}</span>
        </button>
        <button
          onClick={() => setActiveTab('events')}
          className={`pill-tab ${activeTab === 'events' ? 'active' : ''}`}
        >
          系统事件 <span style={{ fontVariantNumeric: "tabular-nums", opacity: 0.7 }}>{events.length}</span>
        </button>
      </div>

      <div className="card" style={{ padding: 0, overflowX: 'auto', borderTopLeftRadius: 0, borderTopRightRadius: 0 }}>
        {loading ? (
          <div style={{ padding: 24, display: "flex", flexDirection: "column", gap: 10 }}>
            {[...Array(5)].map((_, i) => (
              <div key={i} style={{ display: "flex", gap: 16 }}>
                <div className="skeleton" style={{ width: 120, height: 16 }} />
                <div className="skeleton" style={{ width: 80, height: 16 }} />
                <div className="skeleton" style={{ width: 120, height: 16 }} />
                <div className="skeleton" style={{ width: 70, height: 16 }} />
                <div className="skeleton" style={{ flex: 1, height: 16 }} />
              </div>
            ))}
          </div>
        ) : activeTab === 'checkin' ? (
          <>
            {logs.length > 0 && (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>账号</th>
                    <th>站点</th>
                    <th>状态</th>
                    <th>分类</th>
                    <th>信息</th>
                    <th>建议</th>
                    <th>奖励</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map((log, i) => (
                    <tr key={i}>
                      <td style={{ fontSize: 12, color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {format(new Date(log.created_at), 'MM/dd HH:mm:ss')}
                      </td>
                      <td style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>
                        {log.account_username || `Account #${log.account_id}`}
                      </td>
                      <td style={{ fontSize: 13, color: 'var(--color-text-muted)', fontWeight: 400 }}>
                        {log.site_name || '-'}
                      </td>
                      <td>
                        <span className={`badge ${log.status === 'success' ? 'badge-success' : log.status === 'failed' ? 'badge-error' : 'badge-muted'}`}>
                          {log.status === 'success' ? '成功' : log.status === 'failed' ? '失败' : '跳过'}
                        </span>
                      </td>
                      <td>
                        {log.failureReason ? (
                          <span className="badge badge-warning">{log.failureReason.title}</span>
                        ) : log.status === 'success' ? (
                          <span className="badge badge-success">正常</span>
                        ) : (
                          <span className="badge badge-muted">-</span>
                        )}
                      </td>
                      <td style={{ maxWidth: 280 }}>
                        <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={log.message}>
                          {log.message}
                        </span>
                      </td>
                      <td style={{ color: 'var(--color-text-secondary)' }}>
                        {log.failureReason?.actionHint || '-'}
                      </td>
                      <td style={{ color: log.reward ? 'var(--color-success)' : 'inherit' }}>
                        {log.reward || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {logs.length === 0 && (
              <div className="empty-state">
                <svg className="empty-state-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div className="empty-state-title">暂无签到日志</div>
              </div>
            )}
          </>
        ) : (
          <>
            {events.length > 0 && (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>类型</th>
                    <th>级别</th>
                    <th>标题</th>
                    <th>消息</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map((ev, i) => (
                    <tr key={i}>
                      <td style={{ fontSize: 12, color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {format(new Date(ev.created_at), 'MM/dd HH:mm:ss')}
                      </td>
                      <td>
                        <span className="badge badge-info">{ev.type}</span>
                      </td>
                      <td>
                        <span className={`badge ${ev.level === 'error' ? 'badge-error' : ev.level === 'warning' ? 'badge-warning' : 'badge-success'}`}>
                          {ev.level}
                        </span>
                      </td>
                      <td style={{ fontWeight: 500, color: 'var(--color-text-primary)' }}>
                        {ev.title}
                      </td>
                      <td style={{ maxWidth: 400 }}>
                        <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: 'var(--color-text-secondary)' }} title={ev.message}>
                          {ev.message}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {events.length === 0 && (
              <div className="empty-state">
                <svg className="empty-state-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div className="empty-state-title">暂无系统事件</div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
