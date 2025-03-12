use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::{thread, time};
use crate::settings::{Backend, BackendIdentifier, Setting};

#[derive(Default)]
pub struct AvailableBackends {
    pub up: Vec<BackendIdentifier>,
    pub available: Vec<BackendIdentifier>
}



pub fn get_router_config(setting: &Setting) -> Arc<Mutex<AvailableBackends>> {
    let available_backends = Arc::new(Mutex::new(AvailableBackends::default()));
    check_availability(&setting.backends,  &setting.max_pads_per_instance);
    let available_backends_2 = available_backends.clone();
    thread::spawn(move || {
        let ten_millis = time::Duration::from_millis(setting.check_interval);
        loop {
            thread::sleep(ten_millis);
            let new_available_backends = check_availability(&setting.backends, &setting
                .max_pads_per_instance);
            {
                let mut available_backends = available_backends_2.clone().lock().unwrap();
                available_backends.available = new_available_backends.available;
                available_backends.up = new_available_backends.up;
            }
        }
    });
    available_backends
}

pub fn check_availability(backends: &HashMap<BackendIdentifier, Backend>, max_pads_per_instance:
&u32) -> AvailableBackends {
    let mut available = backends.keys().cloned().collect::<Vec<BackendIdentifier>>();
    let mut up = backends.keys().cloned().collect::<Vec<BackendIdentifier>>();
    for (id, backend) in backends.iter() {
        match backend.get_stat() {
            Ok(stat)=>{
                let active_pads = stat.active_pads.unwrap_or(0);
                if active_pads < *max_pads_per_instance {
                    log::info!("Backend {} is up and has {} active pads", id, active_pads);
                } else {
                    available = available.iter().filter(|b| *b != id).cloned().collect();
                }
            }
            Err(e)=>{
                available = available.iter().filter(|b| *b != id).cloned().collect();
                up = up.iter().filter(|b| *b != id).cloned().collect();
            }
        }

    }
    AvailableBackends{
        available,
        up
    }
}