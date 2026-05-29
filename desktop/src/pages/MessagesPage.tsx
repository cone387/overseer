import { useState, useEffect, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";

interface NotificationEntry {
  id: string;
  title: string;
  body: string;
  url: string;
  channel: string;
  source: string;
  level: string;
  received_at: string;
  unread: boolean;
  acked_at?: string;
  expires_at?: string;
}

export default function MessagesPage() {
  const [messages, setMessages] = useState<NotificationEntry[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const loadMessages = useCallback(async () => {
    try {
      const msgs = await invoke<NotificationEntry[]>("get_messages", {
        limit: 50,
      });
      setMessages(msgs);
      const count = await invoke<number>("get_unread_count");
      setUnreadCount(count);
    } catch (e) {
      console.error("Failed to load messages:", e);
    }
  }, []);

  useEffect(() => {
    loadMessages();

    // Listen for new push events to refresh the list
    const unlisten = listen("ws-push", () => {
      loadMessages();
    });

    // Listen for ack_sync events
    const unlistenAck = listen("ws-ack-sync", () => {
      loadMessages();
    });

    // Listen for show-notification (grouper output)
    const unlistenNotif = listen("show-notification", () => {
      loadMessages();
    });

    // Listen for refresh when window is re-shown
    const unlistenRefresh = listen("refresh-messages", () => {
      loadMessages();
    });

    return () => {
      unlisten.then((fn) => fn());
      unlistenAck.then((fn) => fn());
      unlistenNotif.then((fn) => fn());
      unlistenRefresh.then((fn) => fn());
    };
  }, [loadMessages]);

  const handleMarkAllRead = async () => {
    try {
      await invoke("mark_all_read");
      await loadMessages();
    } catch (e) {
      console.error("Failed to mark all read:", e);
    }
  };

  const handleAck = async (id: string) => {
    try {
      await invoke("ack_message", { id });
      await loadMessages();
    } catch (e) {
      console.error("Failed to ack:", e);
    }
  };

  const handleSnooze = async (id: string, duration: string) => {
    try {
      await invoke("snooze_message", { id, duration });
    } catch (e) {
      console.error("Failed to snooze:", e);
    }
  };

  const handleMarkRead = async (id: string) => {
    try {
      await invoke("mark_read", { id });
      await loadMessages();
    } catch (e) {
      console.error("Failed to mark read:", e);
    }
  };

  const handleOpenUrl = (url: string) => {
    if (url) {
      window.open(url, "_blank");
    }
  };

  const formatTime = (isoStr: string) => {
    if (!isoStr) return "";
    try {
      const date = new Date(isoStr);
      if (isNaN(date.getTime())) return isoStr;
      const now = new Date();
      const diff = now.getTime() - date.getTime();

      if (diff < 60000) return "刚刚";
      if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`;
      if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`;

      return date.toLocaleDateString("zh-CN", {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return isoStr || "";
    }
  };

  const getLevelClass = (level: string) => {
    if (level === "critical") return "critical";
    if (level === "timeSensitive") return "timeSensitive";
    return "";
  };

  return (
    <div className="container">
      <div className="header">
        <h1>
          消息列表
          {unreadCount > 0 && <span className="badge">{unreadCount}</span>}
        </h1>
        <div className="toolbar">
          {unreadCount > 0 && (
            <button className="btn btn-secondary" onClick={handleMarkAllRead}>
              全部已读
            </button>
          )}
        </div>
      </div>

      <div className="message-list">
        {messages.length === 0 ? (
          <div className="empty-state">
            <div className="icon">📭</div>
            <p>暂无通知</p>
          </div>
        ) : (
          messages.map((msg) => (
            <div
              key={msg.id}
              className={`message-item ${msg.unread ? "unread" : ""}`}
              onClick={() => {
                setSelectedId(selectedId === msg.id ? null : msg.id);
                if (msg.unread) {
                  handleMarkRead(msg.id);
                }
              }}
            >
              <div className="msg-header">
                <span className="msg-title">{msg.title}</span>
                <span className="msg-time">{formatTime(msg.received_at)}</span>
              </div>
              {msg.body && <div className="msg-body">{msg.body}</div>}
              <div className="msg-meta">
                {msg.channel && (
                  <span className={`msg-tag ${getLevelClass(msg.level)}`}>
                    {msg.channel}
                  </span>
                )}
                {msg.source && <span className="msg-tag">{msg.source}</span>}
                {msg.acked_at && <span className="msg-tag">✓ 已确认</span>}
              </div>

              {selectedId === msg.id && (
                <div className="message-actions">
                  <button
                    className="btn btn-primary"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleAck(msg.id);
                    }}
                  >
                    确认
                  </button>
                  <button
                    className="btn btn-secondary"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleSnooze(msg.id, "1h");
                    }}
                  >
                    稍后 1h
                  </button>
                  <button
                    className="btn btn-secondary"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleSnooze(msg.id, "tomorrow_9am");
                    }}
                  >
                    明早 9 点
                  </button>
                  {msg.url && (
                    <button
                      className="btn btn-secondary"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleOpenUrl(msg.url);
                      }}
                    >
                      打开
                    </button>
                  )}
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
