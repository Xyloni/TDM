<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/app';
	import type { FarmingReport } from '$lib/api/types';
	import PageHead from '$lib/components/PageHead.svelte';
	import PlatformMark from '$lib/components/PlatformMark.svelte';
	import { clock } from '$lib/format';
	import { prefs } from '$lib/prefs.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { farmReasonKeys, problemAccount, sortFarming } from '$lib/farming';

	let report = $state<FarmingReport | null>(null);
	let failed = $state(false);
	let busy = $state(false);
	let search = $state('');
	let group = $state('active');
	let sort = $state('slowest');
	let problems = $state(false);
	let limit = $state(50);
	const groups = $derived([...new Set(report?.accounts.map(a => a.group) ?? [])].sort((a,b) => a-b));
	const rows = $derived(sortFarming((report?.accounts ?? []).filter(a =>
		(!search || `${a.name} ${a.channel} ${a.reward}`.toLowerCase().includes(search.toLowerCase())) &&
		(group === 'all' || (group === 'active' ? !report?.group.id || a.group === report.group.id : a.group === Number(group))) &&
		(!problems || problemAccount(a))
	), sort));
	$effect(() => { search; group; sort; problems; limit = 50; });
	async function refresh() {
		if (busy) return;
		busy = true;
		try { report = await api.farming(); failed = false; } catch { failed = true; } finally { busy = false; }
	}
	onMount(() => {
		void refresh();
		const timer = setInterval(() => { if (!document.hidden) void refresh(); }, 10000);
		return () => clearInterval(timer);
	});
	const duration = (minutes: number | null) => minutes === null ? t('farm.unknown') : t('farm.eta_value', { n: minutes });
</script>

<svelte:head><title>{t('farm.title')} · Pzhomba Pump</title></svelte:head>
<PageHead title={t('farm.title')}>
	{#snippet action()}<button class="btn" onclick={() => void refresh()} disabled={busy}>{t('farm.refresh')}</button>{/snippet}
	<p class="help">{t('farm.description')}</p>
	{#if failed}<p class="error" role="alert">{t('farm.load_error')}</p>{/if}
	{#if report}
		<section class="forecast" aria-label={t('farm.forecast')}>
			<div><span class="eyebrow">{t('farm.forecast')}</span><h2>{report.group.id ? t('farm.group_label', { n: report.group.id }) : t('farm.all_enabled')}</h2><p>{report.group.campaign}</p></div>
			<div><strong>{report.group.completed} / {report.group.total}</strong><span>{t('farm.finished')}</span></div>
			<div><strong>{duration(report.group.eta_minutes)}</strong><span>{report.group.estimated_at ? t('farm.finish_at', { time: clock(report.group.estimated_at, prefs.zone, prefs.fmt) }) : t('farm.unknown_accounts', { n: report.group.unknown })}</span></div>
		</section>
		<p class="help">{t('farm.forecast_help')}</p>
		{#if report.group.delayers.length}
			<details><summary>{t('farm.delayers')}</summary><ul>{#each report.group.delayers.slice(0, 5) as a (a.id)}<li>{a.name} · {duration(a.eta_minutes)} · {t(farmReasonKeys[a.reason] ?? 'farm.unknown')}</li>{/each}</ul></details>
		{/if}
		<div class="toolbar">
			<input aria-label={t('farm.search')} placeholder={t('farm.search')} bind:value={search} />
			<select aria-label={t('farm.group_filter')} bind:value={group}>
				<option value="active">{t('farm.active_group')}</option><option value="all">{t('farm.all_accounts')}</option>
				{#each groups as n (String(n))}<option value={String(n)}>{t('farm.group_label', { n })}</option>{/each}
			</select>
			<select aria-label={t('farm.sort')} bind:value={sort}><option value="slowest">{t('farm.sort_slowest')}</option><option value="eta">{t('farm.sort_eta')}</option><option value="name">{t('farm.sort_name')}</option></select>
			<label><input type="checkbox" bind:checked={problems} />{t('farm.problems_only')}</label>
		</div>
		<div class="table-wrap" class:stale={failed}>
			<table><thead><tr><th>{t('farm.account')}</th><th>{t('farm.progress15')}</th><th>{t('farm.progress30')}</th><th>{t('farm.channel')}</th><th>{t('farm.reason')}</th><th>{t('farm.remaining')}</th></tr></thead>
			<tbody>{#each rows.slice(0, limit) as a (a.id)}
				<tr class:problem={problemAccount(a)}>
					<td><div class="account"><PlatformMark platform={a.platform} /><strong>{a.name}</strong></div><small>{a.reward || t('farm.no_reward')}</small></td>
					{#each [a.stats.minutes_15, a.stats.minutes_30] as w, i (i)}<td title={t('farm.observed', { n: w.observation_minutes.toFixed(1) })}><strong>+{w.gain}</strong><small>{w.rate === null ? t('farm.unknown') : `${Math.round(w.rate * 100)}%`}{!w.ready ? ' · ' + t('farm.partial') : ''}</small></td>{/each}
					<td>{a.channel || '—'}</td><td><span class:bad={problemAccount(a)}>{t(farmReasonKeys[a.reason] ?? 'farm.unknown')}</span>{#if a.waiting_seconds > 0}<small>{t('farm.waiting_duration', { time: `${Math.floor(a.waiting_seconds / 60)}:${String(a.waiting_seconds % 60).padStart(2, '0')}` })}</small>{/if}{#if a.stats.rotations_15}<small>{t('farm.rotations', { n: a.stats.rotations_15 })}</small>{/if}</td><td>{duration(a.eta_minutes)}</td>
				</tr>
			{/each}</tbody></table>
			{#if !rows.length}<p class="empty">{t('farm.empty')}</p>{/if}
		</div>
		{#if rows.length > limit}<button class="btn more" onclick={() => limit += 50}>{t('farm.show_more')}</button>{/if}
		<p class="help">{t('farm.summary', { n: rows.length, observed: report.observed, slow: report.slow })} · {clock(report.at, prefs.zone, prefs.fmt)}</p>
	{:else if !failed}<p class="help">{t('farm.loading')}</p>{/if}
</PageHead>

<style>
	.help { color: var(--muted); font-size: 13px; margin: 12px 0 20px; line-height: 1.6; }
	.forecast { display: flex; flex-wrap: wrap; align-items: center; gap: 28px; padding: 22px; background: var(--surface); border: 1px solid var(--border); border-radius: 10px; }
	.forecast > div:first-child { flex: 1; min-width: 180px; } .forecast strong { display: block; font-size: 23px; } .forecast span, .forecast p { color: var(--muted); font-size: 13px; } .forecast h2 { margin: 5px 0; font-size: 19px; } .forecast p { margin: 0; }
	details { margin: 14px 0 24px; } summary { cursor: pointer; } li { margin: 8px 0; }
	.toolbar { display: flex; gap: 12px; flex-wrap: wrap; margin: 22px 0 16px; align-items: center; } .toolbar > input { flex: 1; min-width: 180px; } .toolbar input, select { background: var(--surface); border: 1px solid var(--border); border-radius: 6px; color: var(--text); padding: 9px; } label { display: flex; align-items: center; gap: 6px; font-size: 13px; }
	.table-wrap { overflow-x: auto; border: 1px solid var(--border); border-radius: 8px; } table { width: 100%; border-collapse: collapse; font-size: 13px; } th, td { padding: 14px 12px; text-align: left; border-bottom: 1px solid var(--border-soft); } th { color: var(--muted); white-space: nowrap; } td { vertical-align: top; } small { display: block; color: var(--muted); margin-top: 6px; max-width: 220px; } .account { display: flex; align-items: center; gap: 8px; } .problem { background: var(--amber-soft); } .bad { color: var(--amber); } .error { color: var(--red); } .stale { opacity: .6; } .empty { padding: 24px; color: var(--muted); } .more { margin-top: 16px; }
</style>
