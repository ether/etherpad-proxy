use std::convert::Infallible;
use std::fmt::format;
use std::net::SocketAddr;
use hyper::Server;
use hyper::server::conn::AddrStream;
use hyper::service::{make_service_fn, service_fn};
use crate::db::DB;
use crate::logging::init_logging;
use crate::reverse_proxy::handle;
use crate::runtime::get_router_config;
use crate::settings::Setting;

mod settings;
mod runtime;
mod stat;
mod logging;
mod error;
mod db;
mod reverse_proxy;

#[tokio::main]
async fn main() {
    init_logging();
    let setting = Setting::try_new().expect("failed to load settings");

    let addr:SocketAddr = format!("0.0.0.0:{}", setting.port).parse().expect("Could not parse ip:port\
    .");

    let data = get_router_config(&setting);
    let db = DB::new(&setting.db_settings.filename);
    let make_svc = make_service_fn(|conn: &AddrStream| {
        let remote_addr = conn.remote_addr().ip();
        async move {
            Ok::<_, Infallible>(service_fn(move |req| handle(remote_addr, req,data, db)))
        }
    });
    let server = Server::bind(&addr).serve(make_svc);
    log::info!("Running server on {:?}", addr);

    if let Err(e) = server.await {
        log::info!("server error: {}", e);
    }
}
