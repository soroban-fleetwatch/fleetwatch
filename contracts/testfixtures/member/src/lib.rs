#![no_std]

use soroban_sdk::{contract, contractimpl, symbol_short, Address, BytesN, ContractExecutable, Env};

#[contract]
pub struct Member;

#[contractimpl]
impl Member {
    pub fn __constructor(env: Env, admin: Address) {
        env.storage()
            .instance()
            .set(&symbol_short!("admin"), &admin);
    }

    pub fn version() -> u32 {
        if cfg!(feature = "v2") {
            2
        } else {
            1
        }
    }

    pub fn repoint(env: Env, wasm_hash: BytesN<32>) {
        let admin: Address = env
            .storage()
            .instance()
            .get(&symbol_short!("admin"))
            .unwrap();
        admin.require_auth();
        env.deployer()
            .update_current_contract(ContractExecutable::Wasm(wasm_hash));
    }
}
