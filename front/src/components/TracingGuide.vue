<template>
    <div class="tracing-guide">
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

        <!-- Two ways to send traces. -->
        <v-row class="mt-2">
            <v-col cols="12" md="6">
                <v-card outlined class="path-card fill-height">
                    <div class="d-flex align-center mb-1">
                        <span class="text-h6">Automatic</span>
                        <v-chip x-small color="primary" class="ml-2">Recommended</v-chip>
                    </div>
                    <div class="grey--text mb-3">No code changes. The platform adds OpenTelemetry to the app when it deploys.</div>
                    <div class="mb-3">
                        <v-chip v-for="l in autoLanguages" :key="l.id" small outlined class="mr-1 mb-1">{{ l.name }}</v-chip>
                    </div>
                    <ol class="steps">
                        <li>Open the app in StackBlaze, then <b>Settings → Tracing</b>.</li>
                        <li>Turn Tracing on and pick the app's language.</li>
                        <li>Save. The app restarts once and its traces appear here.</li>
                    </ol>
                    <v-btn v-if="stackblazeUrl" :href="stackblazeUrl" color="primary" small class="mt-2">Open StackBlaze</v-btn>
                </v-card>
            </v-col>
            <v-col cols="12" md="6">
                <v-card outlined class="path-card fill-height">
                    <div class="text-h6 mb-1">OpenTelemetry SDK</div>
                    <div class="grey--text mb-3">Any language, or when you want custom spans. You add the SDK; the platform does the wiring.</div>
                    <div class="mb-3">
                        <v-chip v-for="l in sdkOnlyLanguages" :key="l.id" small outlined class="mr-1 mb-1">{{ l.name }}</v-chip>
                        <v-chip small outlined class="mr-1 mb-1">and more</v-chip>
                    </div>
                    <ol class="steps">
                        <li>In the app's <b>Settings → Tracing</b>, choose <b>Other: my own OpenTelemetry SDK</b>.</li>
                        <li>Add the SDK to your code with one of the examples below.</li>
                        <li>Deploy. Endpoint, protocol and service name are set for you.</li>
                    </ol>
                </v-card>
            </v-col>
        </v-row>

        <!-- Per-language examples. -->
        <div class="text-h6 mt-6 mb-2">Examples</div>
        <v-card outlined>
            <v-tabs v-model="tab" show-arrows height="44" slider-size="2">
                <v-tab v-for="l in languages" :key="l.id">{{ l.name }}</v-tab>
            </v-tabs>
            <v-divider />
            <v-tabs-items v-model="tab">
                <v-tab-item v-for="l in languages" :key="l.id" transition="none">
                    <div class="pa-4">
                        <v-alert v-if="l.auto" dense text color="success" icon="mdi-check-circle-outline" class="mb-4">
                            {{ l.name }} works without code: turn Tracing on and pick <b>{{ l.name }}</b
                            >. Use the SDK below only for custom spans.
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

        <!-- What the platform sets, for reference. -->
        <div class="text-h6 mt-6 mb-2">Set for you</div>
        <div class="grey--text mb-2">
            With <b>Other: my own OpenTelemetry SDK</b>, the app gets these variables. Every OpenTelemetry SDK reads them, so you do not configure an
            exporter.
        </div>
        <Code>
            <pre>{{ envBlock }}</pre>
        </Code>
        <div class="grey--text text-caption">
            <v-icon small class="mr-1">mdi-shield-lock-outline</v-icon>
            No API key needed. The platform accepts traces only from apps with Tracing on and keeps them private to your workspace.
        </div>
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
        autoLanguages() {
            return this.languages.filter((l) => l.auto);
        },
        sdkOnlyLanguages() {
            return this.languages.filter((l) => !l.auto);
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
    max-width: 1100px;
}
.guide-hero {
    display: flex;
    align-items: center;
    padding: 20px 0 8px;
}
.path-card {
    padding: 20px;
}
.steps {
    padding-left: 20px;
}
.steps li {
    margin-bottom: 6px;
}
.tracing-guide :deep(.code pre) {
    padding-bottom: 20px;
    white-space: pre;
}
</style>
