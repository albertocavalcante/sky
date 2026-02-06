import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLinksValidator from 'starlight-links-validator';
// TODO: Re-enable once published to npm or GitHub Packages
// Currently disabled due to gitpkg.vercel.app rate limits (402 error)
// import starlightSidebarOrphanPages from '@albertocavalcante/starlight-sidebar-orphan-pages';
import sitemap from '@astrojs/sitemap';
import tailwindcss from '@tailwindcss/vite';
import fs from 'node:fs';

// Load TextMate grammars for syntax highlighting
const starlarkGrammar = JSON.parse(
	fs.readFileSync(new URL('./starlark.tmLanguage.json', import.meta.url), 'utf-8')
);
const textprotoGrammar = JSON.parse(
	fs.readFileSync(new URL('./textproto.tmLanguage.json', import.meta.url), 'utf-8')
);

export default defineConfig({
	site: 'https://albertocavalcante.github.io',
	base: '/sky',

	vite: {
		plugins: [tailwindcss()],
	},

	integrations: [
		sitemap(),
		starlight({
			plugins: [
				starlightLinksValidator(),
				// TODO: Re-enable once published to npm or GitHub Packages
				// starlightSidebarOrphanPages({ failOnOrphans: true }),
			],
			title: 'Sky',
			description: 'A comprehensive toolkit for Starlark development',
			favicon: '/favicon.svg',
			lastUpdated: true,
			defaultLocale: 'root',
			locales: {
				root: {
					label: 'English',
					lang: 'en',
				},
				// Future locales:
				// pt: { label: 'Português', lang: 'pt' },
				// es: { label: 'Español', lang: 'es' },
			},
			tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
			expressiveCode: {
				themes: ['github-dark', 'github-light'],
				shiki: {
					langs: [starlarkGrammar, textprotoGrammar],
					langAlias: {
						bzl: 'starlark',
						bazel: 'starlark',
						build: 'starlark',
						star: 'starlark',
						pbtxt: 'textproto',
						prototext: 'textproto',
						txtpb: 'textproto',
					},
				},
				styleOverrides: {
					borderRadius: '0.625rem',
					codeFontFamily: "'JetBrains Mono', 'SF Mono', 'Fira Code', ui-monospace, monospace",
					codeFontSize: '0.875rem',
					codeLineHeight: '1.65',
				},
			},
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/albertocavalcante/sky' },
			],
			editLink: {
				baseUrl: 'https://github.com/albertocavalcante/sky/edit/main/docs/website/',
			},
			customCss: ['./src/styles/global.css'],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'getting-started/introduction' },
						{ label: 'Installation', slug: 'getting-started/installation' },
						{ label: 'Quick Start', slug: 'getting-started/quick-start' },
					],
				},
				{
					label: 'Starlark Language',
					items: [
						{ label: 'What is Starlark?', slug: 'starlark/overview' },
						{ label: 'Language Basics', slug: 'starlark/basics' },
						{ label: 'Type System', slug: 'starlark/types' },
						{ label: 'Type System Roadmap', slug: 'starlark/types-roadmap' },
						{ label: 'Implementations', slug: 'starlark/implementations' },
						{ label: 'Best Practices', slug: 'starlark/best-practices' },
						{ label: 'Tooling & IDE Support', slug: 'starlark/tooling' },
						{ label: 'Resources', slug: 'starlark/resources' },
					],
				},
				{
					label: 'Starlark in Practice',
					items: [
						{ label: 'Use Cases Overview', slug: 'starlark/use-cases' },
					{ label: 'Bazel', slug: 'starlark/bazel' },
					{ label: 'Buck2', slug: 'starlark/buck2' },
						{ label: 'Bazel vs Buck2', slug: 'starlark/build-systems-comparison' },
						{ label: 'Skycfg', slug: 'starlark/skycfg-deep-dive' },
						{ label: 'ytt', slug: 'starlark/ytt-deep-dive' },
						{ label: 'Tilt', slug: 'starlark/tilt' },
						{ label: 'Aspect CLI & AXL', slug: 'starlark/aspect-cli-axl' },
					],
				},
				{
					label: 'Tools',
					items: [
						{ label: 'Overview', slug: 'tools/overview' },
						{ label: 'skylint', slug: 'tools/skylint' },
						{ label: 'skyfmt', slug: 'tools/skyfmt' },
						{ label: 'skytest', slug: 'tools/skytest' },
						{ label: 'skycov', slug: 'tools/skycov' },
						{ label: 'skydoc', slug: 'tools/skydoc' },
						{ label: 'skycheck', slug: 'tools/skycheck' },
						{ label: 'skyquery', slug: 'tools/skyquery' },
						{ label: 'skyrepl', slug: 'tools/skyrepl' },
					],
				},
				{
					label: 'Plugins',
					items: [
						{ label: 'Overview', slug: 'plugins' },
						{ label: 'Quick Start', slug: 'plugins/quick-start' },
						{ label: 'Protocol', slug: 'plugins/protocol' },
						{ label: 'Native Plugins', slug: 'plugins/native' },
						{ label: 'WASM Plugins', slug: 'plugins/wasm' },
						{ label: 'SDK Reference', slug: 'plugins/sdk' },
						{ label: 'Testing', slug: 'plugins/testing' },
						{ label: 'Publishing', slug: 'plugins/publishing' },
					],
				},
				{
					label: 'Coverage',
					items: [
						{ label: 'Getting Started', slug: 'coverage/getting-started' },
						{ label: 'Output Formats', slug: 'coverage/output-formats' },
						{ label: 'CI Integration', slug: 'coverage/ci-integration' },
						{ label: 'IDE Integration', slug: 'coverage/ide-integration' },
					],
				},
				{
					label: 'starlark-go-x',
					items: [
						{ label: 'Overview', slug: 'starlark-go-x/overview' },
						{ label: 'Coverage Hooks API', slug: 'starlark-go-x/hooks' },
						{ label: 'Technical Deep Dive', slug: 'starlark-go-x/technical' },
						{ label: 'Type Annotations', slug: 'starlark-go-x/type-annotations' },
						{ label: 'Roadmap & TODO', slug: 'starlark-go-x/roadmap' },
					],
				},
				{
					label: 'Language Server',
					items: [
						{ label: 'Overview', slug: 'lsp' },
						{ label: 'Custom Dialects', slug: 'lsp/custom-dialects' },
						{ label: 'Builtin Formats', slug: 'lsp/builtin-formats' },
					],
				},
				{
					label: 'Reference',
					autogenerate: { directory: 'reference' },
				},
			],
		}),
	],
});
