use crate::db::DB;
use crate::logging::init_logging;
use crate::reverse_proxy::{handler, StateOfReverseProxy};
use crate::runtime::get_router_config;
use crate::settings::Setting;
use axum::body::Body;
use axum::extract::Request;
use axum::http::Response;
use axum::routing::any_service;
use axum::Router;
use hyper_util::client::legacy::connect::HttpConnector;
use hyper_util::rt::TokioExecutor;
use std::convert::Infallible;
use std::sync::{Arc, Mutex};
use axum::handler::Handler;
use tower::service_fn;

mod settings;
mod runtime;
mod stat;
mod logging;
mod error;
mod db;
mod reverse_proxy;

type Client = hyper_util::client::legacy::Client<HttpConnector, Body>;

#[tokio::main]
async fn main() {
    init_logging();
    let setting = Setting::try_new().expect("failed to load settings");
    let client: Client =
        hyper_util::client::legacy::Client::<(), ()>::builder(TokioExecutor::new())
            .build(HttpConnector::new());
    let data = get_router_config(&setting).await;

    let arc_db = Arc::new(Mutex::new(DB::new(&setting.db_settings.filename)));
    let state = StateOfReverseProxy{
        available_backends: data,
        db: arc_db,
        client,
        setting: setting.clone(),
    };
    let app = Router::new().fallback(handler).with_state(state);

    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{}", setting.port))
        .await
        .unwrap();
    axum::serve(listener, app).await.unwrap();

}
