export const copy = {
  zh: {
    title: '自动切换优质代理', enabled: '开启账号自动切换', loading: '加载中…', save: '保存配置', saving: '保存中…', refresh: '刷新配置', saved: '配置已保存。关闭开关后保留当前代理。',
    description: '仅在所选代理池内按近期质量切换，不定时轮换 IP。默认关闭，关闭后保留当前绑定。',
    rule: '以实际请求的成功和传输失败为依据，按同一账号、同一模型、同一入口类型比较首字耗时。候选需要该账号在近一小时的历史样本及近期连通监测。',
    trigger: '同模型、同入口出现持久传输故障、连续 3 次传输失败或成功率低于 80% 时可切换；连续 3 次 5xx 须有候选近期成功证据。首字耗时同时改善 30% 和 200ms 时可优化。',
    pool: '候选代理（2–10 条，须包含当前代理）', search: '搜索已开启监测的代理', empty: '暂无可选代理，请先在代理管理中开启 OpenAI 监测。', unavailable: '当前不可选',
    hint: '仅列出已开启监测、启用且未过期的非主备代理（最多 500 条）。手工改绑会暂停托管；固定主备不可同时使用。',
    current: '当前代理', status: '最近评估', evaluated: '评估时间', history: '最近切换（最多 20 条）', noHistory: '暂无自动切换记录',
    invalid: '请选择 2–10 条代理并包含当前代理。', error: '请求失败或保存状态未确认，请刷新配置核对后重试。',
    saveForSamples: '保存后查看样本', noSamples: '近 1 小时暂无样本', sampleScope: '近 1 小时，最多 5 组', unknownTransport: '入口未知，不用于比较',
    sampleCounts: (ok: number, total: number, ttft: number, errors: number) => `成功 ${ok}/${total} · 首字 ${ttft} · 5xx ${errors}`,
    metric: '样本不足时保持当前代理。候选代理也需要本账号同模型的历史请求；鉴权、配额及无法归因的失败不计入代理故障。',
    states: { disabled: '已关闭', waiting: '等待评估', cooldown: '切换冷却中', current_unknown: '当前代理缺测或暂停，等待有效样本', no_candidate: '暂无达标候选，保持当前代理', healthy: '保持当前代理', degraded: '当前质量下降，已切换', improved: '发现明显改善，已切换', unavailable: '当前代理不可用，已切换', business_insufficient: '实际请求可比样本不足，保持当前代理', business_stable: '当前代理表现稳定', collection_unavailable: '采集不完整，等待恢复', business_degraded: '实际请求传输失败，已切换', business_5xx: '连续 5xx 且候选近期成功，已切换', business_improved: '实际请求首字耗时改善，已切换', manual_override: '手工改绑，已暂停托管', account_unavailable: '账号不可用', conflict: '代理池或主备模式冲突', evaluation_error: '评估失败，稍后重试' }
  },
  en: {
    title: 'Automatic proxy quality routing', enabled: 'Enable for this account', loading: 'Loading…', save: 'Save configuration', saving: 'Saving…', refresh: 'Reload configuration', saved: 'Saved. Disabling keeps the current proxy.',
    description: 'Switch within the selected pool using recent quality. No timed IP rotation. Disabled by default; disabling keeps the current binding.',
    rule: 'Use successful requests and confirmed transport failures. Compare time to first token for the same account, model and ingress type. Candidates need recent connectivity checks and actual request history from this account within the last hour.',
    trigger: 'Switch on persistent transport faults, 3 consecutive transport failures or success below 80% in the same model and ingress. Three consecutive 5xx responses also require recent candidate successes. Improve for at least 30% and 200ms faster first token.',
    pool: 'Proxy pool (2–10, including the current proxy)', search: 'Search monitored proxies', empty: 'No candidates. Enable OpenAI monitoring in proxy management first.', unavailable: 'Unavailable',
    hint: 'Lists up to 500 active, unexpired, monitored proxies without fixed failover. Manual reassignment pauses routing; fixed failover cannot run alongside it.',
    current: 'Current proxy', status: 'Latest decision', evaluated: 'Evaluated at', history: 'Recent switches (up to 20)', noHistory: 'No automatic switches yet',
    invalid: 'Select 2–10 proxies, including the current proxy.', error: 'Request failed or save outcome is unknown. Reload configuration before retrying.',
    saveForSamples: 'Save to view samples', noSamples: 'No samples in the past hour', sampleScope: 'Past hour, up to 5 groups', unknownTransport: 'Unknown ingress, excluded',
    sampleCounts: (ok: number, total: number, ttft: number, errors: number) => `Success ${ok}/${total} · TTFT ${ttft} · 5xx ${errors}`,
    metric: 'Insufficient samples keep the current proxy. Candidates also require same-account, same-model history; authentication, quota and unclassified failures are not treated as proxy faults.',
    states: { disabled: 'Disabled', waiting: 'Waiting', cooldown: 'Cooling down', current_unknown: 'Current data missing or monitoring paused', no_candidate: 'No qualified candidate; keeping current proxy', healthy: 'Keeping current proxy', degraded: 'Switched after degradation', improved: 'Switched for significant improvement', unavailable: 'Switched from unavailable proxy', business_insufficient: 'Insufficient comparable request samples', business_stable: 'Keeping the current proxy', collection_unavailable: 'Collection incomplete; waiting', business_degraded: 'Switched after transport failures', business_5xx: 'Switched after 5xx with recent candidate success', business_improved: 'Switched for faster first token', manual_override: 'Paused after manual reassignment', account_unavailable: 'Account unavailable', conflict: 'Pool or fixed failover conflict', evaluation_error: 'Evaluation failed; retrying later' }
  }
}
