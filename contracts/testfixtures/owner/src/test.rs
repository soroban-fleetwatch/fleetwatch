use super::*;
use soroban_sdk::{testutils::Address as _, Env, Executable};

mod member {
    soroban_sdk::contractimport!(file = "../target/fixtures/member-v1.wasm");
}
const V1: &[u8] = include_bytes!("../../target/fixtures/member-v1.wasm");
const V2: &[u8] = include_bytes!("../../target/fixtures/member-v2.wasm");
const OWNER: &[u8] = include_bytes!("../../target/fixtures/owner.wasm");

#[test]
fn shared_tag_changes_both_members_and_direct_repoint_leaves_fleet() {
    let env = Env::default();
    env.mock_all_auths();
    let admin = Address::generate(&env);
    let owner_id = env.register(OWNER, (&admin,));
    let owner = OwnerClient::new(&env, &owner_id);
    let a = env.deployer().upload_contract_wasm(V1);
    let b = env.deployer().upload_contract_wasm(V2);
    assert_ne!(a, b);
    let tag = String::from_str(&env, "fleetwatch-demo");
    owner.publish(&tag, &a);
    let first = owner.deploy(&tag, &BytesN::from_array(&env, &[1; 32]));
    let second = owner.deploy(&tag, &BytesN::from_array(&env, &[2; 32]));
    assert_ne!(first, second);
    let first_client = member::Client::new(&env, &first);
    let second_client = member::Client::new(&env, &second);
    assert_eq!(first_client.version(), 1);
    assert_eq!(second_client.version(), 1);
    owner.publish(&tag, &b);
    assert_eq!(first_client.version(), 2);
    assert_eq!(second_client.version(), 2);
    assert_eq!(first.executable(), Some(Executable::Wasm(b.clone())));
    assert_eq!(second.executable(), Some(Executable::Wasm(b)));
    // Return to A and verify that an intermediate version was actually executed.
    owner.publish(&tag, &a);
    assert_eq!(first_client.version(), 1);
    assert_eq!(second_client.version(), 1);
    // A direct repoint of one member detaches it from subsequent shared updates.
    first_client.repoint(&a);
    owner.publish(&tag, &env.deployer().upload_contract_wasm(V2));
    assert_eq!(first_client.version(), 1);
    assert_eq!(second_client.version(), 2);
}

#[test]
fn publisher_and_factory_require_administrator_authorization() {
    let env = Env::default();
    let admin = Address::generate(&env);
    let owner_id = env.register(OWNER, (&admin,));
    let owner = OwnerClient::new(&env, &owner_id);
    let tag = String::from_str(&env, "fleetwatch-demo");
    let a = env.deployer().upload_contract_wasm(V1);
    assert!(owner.try_publish(&tag, &a).is_err());
    assert!(owner
        .try_deploy(&tag, &BytesN::from_array(&env, &[1; 32]))
        .is_err());
}

#[test]
fn member_repoint_requires_administrator_authorization() {
    let env = Env::default();
    let admin = Address::generate(&env);
    let id = env.register(V1, (&admin,));
    let member = member::Client::new(&env, &id);
    let b = env.deployer().upload_contract_wasm(V2);
    assert!(member.try_repoint(&b).is_err());
    assert_eq!(member.version(), 1);
}
