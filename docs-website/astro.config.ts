import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLinksValidator from 'starlight-links-validator';
import sitemap from '@astrojs/sitemap';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
	site: 'https://albertocavalcante.github.io',
	base: '/sky',

	vite: {
		plugins: [tailwindcss()],
	},

	integrations: [
		sitemap(),
		starlight({
			plugins: [starlightLinksValidator()],
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
				baseUrl: 'https://github.com/albertocavalcante/sky/edit/main/docs-website/',
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
						{ label: 'Resources', slug: 'starlark/resources' },
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
					label: 'Coverage',
					items: [
						{ label: 'Getting Started', slug: 'coverage/getting-started' },
						{ label: 'Output Formats', slug: 'coverage/output-formats' },
						{ label: 'CI Integration', slug: 'coverage/ci-integration' },
					],
				},
				{
					label: 'starlark-go-x',
					items: [
						{ label: 'Overview', slug: 'starlark-go-x/overview' },
						{ label: 'OnExec Hook', slug: 'starlark-go-x/onexec-hook' },
						{ label: 'Type Annotations', slug: 'starlark-go-x/type-annotations' },
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
