<template>
    <div class="tracing-guide" :class="{ compact }">
        <!-- Header: what tracing is and that the plan already includes it. -->
        <div v-if="!compact" class="guide-hero">
            <v-icon size="40" color="primary" class="mr-4">mdi-chart-timeline-variant</v-icon>
            <div>
                <div class="text-h5">Start tracing your apps</div>
                <div class="grey--text mt-1">
                    Tracing is included in your plan. Follow each request through your apps: time spent per step, errors, and slow database or HTTP
                    calls.
                </div>
            </div>
        </div>

        <v-row class="mt-2">
            <!-- Left: per-language examples. Below the actions on narrow screens. -->
            <v-col cols="12" :md="compact ? 12 : 7" order="2" :order-md="compact ? 2 : 1">
                <v-card outlined>
                    <div class="text-h6 px-4 pt-3">Examples</div>
                    <v-tabs v-model="tab" show-arrows height="44" slider-size="2">
                        <v-tab v-for="l in languages" :key="l.id">{{ l.name }}</v-tab>
                    </v-tabs>
                    <v-divider />
                    <v-tabs-items v-model="tab">
                        <v-tab-item v-for="l in languages" :key="l.id" transition="none">
                            <div class="pa-4">
                                <v-alert v-if="l.auto" dense text color="success" icon="mdi-check-circle-outline" class="mb-4">
                                    No code needed: pick <b>{{ l.name }}</b> in <b>Settings → Tracing</b>. The SDK below is only for custom spans.
                                </v-alert>
                                <template v-for="(s, i) in l.steps">
                                    <div :key="'t' + i" class="subtitle-2 mb-1">{{ s.title }}</div>
                                    <Code :key="'c' + i">
                                        <pre>{{ s.code }}</pre>
                                    </Code>
                                </template>
                                <a :href="l.docs" target="_blank" rel="noopener"
                                    >{{ l.name }} guide on opentelemetry.io<v-icon small>mdi-open-in-new</v-icon></a
                                >
                            </div>
                        </v-tab-item>
                    </v-tabs-items>
                </v-card>
            </v-col>

            <!-- Right: what to do in StackBlaze. Stays in view while scrolling. -->
            <v-col cols="12" :md="compact ? 12 : 5" order="1" :order-md="compact ? 1 : 2">
                <v-card outlined class="actions">
                    <div class="text-h6 mb-3">Turn it on</div>

                    <div class="path" :class="{ active: activeAuto }">
                        <div class="d-flex align-center">
                            <span class="subtitle-1 font-weight-medium">Automatic</span>
                            <v-chip x-small color="primary" class="ml-2">Recommended</v-chip>
                        </div>
                        <div class="grey--text mb-2">No code changes.</div>
                        <div class="mb-2">
                            <v-chip v-for="l in autoLanguages" :key="l.id" x-small outlined class="mr-1 mb-1">{{ l.name }}</v-chip>
                        </div>
                        <ol class="steps">
                            <li>Open the app in StackBlaze.</li>
                            <li><b>Settings → Tracing</b>: turn it on and pick the language.</li>
                            <li>Save. The app restarts once.</li>
                        </ol>
                        <v-btn v-if="stackblazeUrl" :href="stackblazeUrl" color="primary" small>Open StackBlaze</v-btn>
                    </div>

                    <v-divider class="my-4" />

                    <div class="path" :class="{ active: !activeAuto }">
                        <div class="subtitle-1 font-weight-medium">Your own SDK</div>
                        <div class="grey--text mb-2">Any language, or custom spans.</div>
                        <ol class="steps">
                            <li><b>Settings → Tracing</b>: choose <b>Other: my own OpenTelemetry SDK</b>.</li>
                            <li>Add the SDK using the example<span class="hidden-sm-and-down"> on the left</span>.</li>
                            <li>Deploy.</li>
                        </ol>
                    </div>

                    <v-divider class="my-4" />

                    <div class="subtitle-2 mb-1">Set for you</div>
                    <div class="grey--text text-caption mb-2">Every OpenTelemetry SDK reads these, so you do not configure an exporter.</div>
                    <Code>
                        <pre>{{ envBlock }}</pre>
                    </Code>
                    <div class="grey--text text-caption d-flex">
                        <v-icon small class="mr-1">mdi-shield-lock-outline</v-icon>
                        <span>No API key needed. Traces are accepted only from apps with Tracing on and stay in your workspace.</span>
                    </div>
                </v-card>
            </v-col>
        </v-row>
    </div>
</template>

<script>
import Code from './Code.vue';
import { PLATFORM_OTLP_ENDPOINT, returnUrlOrigin } from '../utils/handoff';

const LANGUAGES = [
    {
        id: 'nodejs',
        name: 'Node.js',
        auto: true,
        docs: 'https://opentelemetry.io/docs/languages/js/getting-started/nodejs/',
        steps: [
            {
                title: 'Install',
                code: 'npm install @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @opentelemetry/exporter-trace-otlp-proto',
            },
            {
                title: 'tracing.js',
                code: `const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-proto');

new NodeSDK({
  traceExporter: new OTLPTraceExporter(),
  instrumentations: [getNodeAutoInstrumentations()],
}).start();`,
            },
            { title: 'Start command', code: 'node --require ./tracing.js app.js' },
        ],
    },
    {
        id: 'python',
        name: 'Python',
        auto: true,
        docs: 'https://opentelemetry.io/docs/languages/python/getting-started/',
        steps: [
            {
                title: 'Install',
                code: `pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install`,
            },
            { title: 'Start command', code: 'opentelemetry-instrument python app.py' },
        ],
    },
    {
        id: 'java',
        name: 'Java',
        auto: true,
        docs: 'https://opentelemetry.io/docs/zero-code/java/agent/getting-started/',
        steps: [
            {
                title: 'Download the agent (in your Dockerfile)',
                code: `ADD https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar /app/`,
            },
            { title: 'Start command', code: 'java -javaagent:/app/opentelemetry-javaagent.jar -jar app.jar' },
        ],
    },
    {
        id: 'dotnet',
        name: '.NET',
        auto: true,
        docs: 'https://opentelemetry.io/docs/languages/dotnet/getting-started/',
        steps: [
            {
                title: 'Install',
                code: `dotnet add package OpenTelemetry.Extensions.Hosting
dotnet add package OpenTelemetry.Instrumentation.AspNetCore
dotnet add package OpenTelemetry.Instrumentation.Http
dotnet add package OpenTelemetry.Exporter.OpenTelemetryProtocol`,
            },
            {
                title: 'Program.cs',
                code: `builder.Services.AddOpenTelemetry()
    .WithTracing(t => t
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddOtlpExporter());`,
            },
        ],
    },
    {
        id: 'go',
        name: 'Go',
        docs: 'https://opentelemetry.io/docs/languages/go/getting-started/',
        steps: [
            {
                title: 'Install',
                code: `go get go.opentelemetry.io/otel \\
  go.opentelemetry.io/otel/sdk \\
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \\
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`,
            },
            {
                title: 'main.go',
                code: `exp, err := otlptracehttp.New(ctx)
if err != nil {
    log.Fatal(err)
}
tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
defer tp.Shutdown(ctx)
otel.SetTracerProvider(tp)

// Trace every incoming HTTP request.
http.ListenAndServe(":8080", otelhttp.NewHandler(mux, "server"))`,
            },
        ],
    },
    {
        id: 'php',
        name: 'PHP',
        docs: 'https://opentelemetry.io/docs/zero-code/php/',
        steps: [
            {
                title: 'Install (extension + SDK)',
                code: `pecl install opentelemetry
echo "extension=opentelemetry.so" > /usr/local/etc/php/conf.d/opentelemetry.ini
composer require open-telemetry/sdk open-telemetry/exporter-otlp php-http/guzzle7-adapter`,
            },
            {
                title: 'Framework instrumentation (pick yours)',
                code: `composer require open-telemetry/opentelemetry-auto-laravel
# or: opentelemetry-auto-symfony, opentelemetry-auto-slim, opentelemetry-auto-wordpress`,
            },
            { title: 'Environment variable', code: 'OTEL_PHP_AUTOLOAD_ENABLED=true' },
        ],
    },
    {
        id: 'ruby',
        name: 'Ruby',
        docs: 'https://opentelemetry.io/docs/languages/ruby/getting-started/',
        steps: [
            {
                title: 'Gemfile',
                code: `gem 'opentelemetry-sdk'
gem 'opentelemetry-exporter-otlp'
gem 'opentelemetry-instrumentation-all'`,
            },
            {
                title: 'config/initializers/opentelemetry.rb',
                code: `require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/all'

OpenTelemetry::SDK.configure(&:use_all)`,
            },
        ],
    },
    {
        id: 'rust',
        name: 'Rust',
        docs: 'https://opentelemetry.io/docs/languages/rust/getting-started/',
        steps: [
            {
                title: 'Cargo.toml',
                code: `opentelemetry = "0.30"
opentelemetry_sdk = "0.30"
opentelemetry-otlp = { version = "0.30", features = ["http-proto", "reqwest-blocking-client"] }`,
            },
            {
                title: 'main.rs',
                code: `let exporter = opentelemetry_otlp::SpanExporter::builder()
    .with_http()
    .build()?;
let provider = opentelemetry_sdk::trace::SdkTracerProvider::builder()
    .with_batch_exporter(exporter)
    .build();
opentelemetry::global::set_tracer_provider(provider.clone());`,
            },
        ],
    },
];

export default {
    components: { Code },

    props: {
        // Inside the dialog: no page header.
        compact: Boolean,
    },

    data() {
        return { tab: 0, languages: LANGUAGES };
    },

    computed: {
        activeAuto() {
            const l = this.languages[this.tab];
            return !l || !!l.auto;
        },
        autoLanguages() {
            return this.languages.filter((l) => l.auto);
        },
        stackblazeUrl() {
            return returnUrlOrigin();
        },
        envBlock() {
            return [
                `OTEL_EXPORTER_OTLP_ENDPOINT=${PLATFORM_OTLP_ENDPOINT}`,
                'OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf',
                'OTEL_SERVICE_NAME=<your app name>',
            ].join('\n');
        },
    },
};
</script>

<style scoped>
.tracing-guide {
    max-width: 1400px;
}
.guide-hero {
    display: flex;
    align-items: center;
    padding: 20px 0 8px;
}
.actions {
    padding: 20px;
}
@media (min-width: 960px) {
    .tracing-guide:not(.compact) .actions {
        position: sticky;
        top: 16px;
    }
}
.path {
    border-left: 3px solid transparent;
    padding-left: 12px;
    margin-left: -15px;
    transition: border-color 0.2s;
}
.path.active {
    border-left-color: var(--v-primary-base);
}
.steps {
    padding-left: 20px;
    margin-bottom: 8px;
}
.steps li {
    margin-bottom: 4px;
}
.tracing-guide :deep(.code pre) {
    padding-bottom: 20px;
    white-space: pre;
}
.actions :deep(.code pre) {
    font-size: 12px;
}
</style>
