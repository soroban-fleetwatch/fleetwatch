#![no_std]

use soroban_sdk::{
    contract, contractimpl, symbol_short, Address, BytesN, ContractExecutable,
    ContractExecutableRef, Env, String,
};

// Test-only fixture. The watcher never holds this contract's administrator key.
#[contract]
pub struct Owner;

fn administrator(env: &Env) -> Address {
    env.storage()
        .instance()
        .get(&symbol_short!("admin"))
        .unwrap()
}

#[contractimpl]
impl Owner {
    pub fn __constructor(env: Env, admin: Address) {
        env.storage()
            .instance()
            .set(&symbol_short!("admin"), &admin);
    }

    pub fn publish(env: Env, tag: String, wasm_hash: BytesN<32>) {
        administrator(&env).require_auth();
        env.executable_refs().set(&tag, &wasm_hash);
    }

    pub fn deploy(env: Env, tag: String, salt: BytesN<32>) -> Address {
        let admin = administrator(&env);
        admin.require_auth();
        env.deployer().with_current_contract(salt).deploy_contract(
            ContractExecutable::ExternalRef(ContractExecutableRef {
                owner: env.current_contract_address(),
                tag,
            }),
            (admin,),
        )
    }
}

#[cfg(test)]
mod test;
