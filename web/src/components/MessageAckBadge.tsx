interface MessageAckBadgeProps {
  ackAt: string | null | undefined;
}

export function MessageAckBadge({ ackAt }: MessageAckBadgeProps) {
  if (ackAt) {
    const time = new Date(ackAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
    return <span style={{ color: '#22c55e', fontSize: '12px', fontWeight: 500 }}>✓ 已确认 {time}</span>;
  }
  return <span style={{ color: '#9ca3af', fontSize: '12px' }}>未确认</span>;
}
