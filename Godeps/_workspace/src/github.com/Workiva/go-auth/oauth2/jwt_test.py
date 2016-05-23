"""
This is a helper file for generating JWTs with a common private key
for testing jwt.go

You can alter the data dictionary to generate a JWT by running:

    python jwt_test.py

A number of example token claims are provided.  These have been
generated directly by iam-services, so they should be assumed accurate.

They have been altered to extend the expiration.

Requirements:
    pip install crypto, PyJWT

"""

import Crypto
import jwt

from jwt.contrib.algorithms.pycrypto import RSAAlgorithm

jwt.register_algorithm('RS512', RSAAlgorithm(RSAAlgorithm.SHA512))


#####################
# Keypair information
#####################

key_tuple = (
    long(24817274073206754688089592776519460949993444832444465314388162966279215222541242370345932006647969582133527832561571271768246177219593612597946389077957724223941590730268297872983736606496903131842239263202422685928604632113817403094928625050040388835058652109541294151564277666894800286197651304640441484131394293710877245812362804237018108331192362259905398454481188527795648400306295307416517142699592543985768442044996504164192500503843801666313632884910727025952779596604561869859910499774223571385622645368847832384519343364333564714339411497698156134852802082548052571415951864868489908018458922239880432536529),
    long(65537),
    long(18287010384108222796240636806135126847385193674541222804864933792629443848924449952679337602652604590695215314861275307936465575023095575622625799492694728446871039253339588947955057573598859362542158147284303467489573445443649694527675864945245440859707530519766974032490686132866681346736301277197555581605170595036508102440564807104557131459961525911328285283603815143800043553563200255989274923615935513052726663259861627594846950626573304436443806425545339373239680499037435170324729887098111542701548398800384779491496310747939430367315930866794439750262909660507211527701974062977118068431713762640280196910417)
)


#############
# Helper data
#############

issuer = "https://wk-dev.wdesk.org"
client_id = "test-client-id"

membership_rid = "TWVtYmVyc2hpcB81NzE5MjMyMzY5MTMxNTIw"
account_rid = "QWNjb3VudB81Njg4NDIyMDE4NTgwNDgw"
user_rid = "V0ZVc2VyHzU3Mjg0MTUzMTUzOTQ1NjA"

mem = 5719232369131520
acc = 5688422018580480
usr = 5728415315394560

valid_exp = 4102444800
expired_exp = 946684800


################
# Example claims
################

v5_data = {
  "context": {
    "username": "mike.davis@workiva.com",
    "csrf_token": None,
    "membership_status": 2,
    "membership": "TWVtYmVyc2hpcB81NzE5MjMyMzY5MTMxNTIw",
    "user": "V0ZVc2VyHzU3Mjg0MTUzMTUzOTQ1NjA",
    "licenses": [
      "section16User",
      "section16"
    ],
    "account_name": "Test Account",
    "permissions": [
      "section16Admin"
    ],
    "account": "QWNjb3VudB81Njg4NDIyMDE4NTgwNDgw",
    "roles": [
      "super-admin|super-admin",
      "super-admin|restricted-admin",
      "super-admin|server-admin-rapid-response",
      "remote-console|remote-console-admin",
      "super-admin|server-admin-software-support-engineer",
      "super-admin|server-admin-customer-success",
      "super-admin|server-admin-viewer",
      "super-admin|release-management",
      "remote-console|remote-console-viewer"
    ],
    "collect_stats": True
  },
  "iat": 1461019146,
  "sub": "TWVtYmVyc2hpcB81NzE5MjMyMzY5MTMxNTIw",
  "par": "2a016105803b7b1b",
  "ver": 5,
  "aud": "test-client-id",
  "gnt": "implicit",
  "iss": "https://wk-dev.wdesk.org",
  "wsk": 0,
  "jti": "17dcba9e0d614f27b054a89fa24365d4",
  "exp": 4102444800,
  "scope": "sox|r sox|w"
}

v4_undelegated_data = {
  "scope": "sox|r sox|w",
  "scp": [
    "sox|r",
    "sox|w"
  ],
  "par": "eb779d2b8ca64c66ad697a07decfc456",
  "ver": 4,
  "aud": "test-client-id",
  "gnt": "urn:ietf:params:oauth:grant-type:jwt-bearer",
  "cid": "test-client-id",
  "iss": "https://wk-dev.wdesk.org",
  "wsk": 1,
  "jti": "615b7f1217884cdfac57da0aba722961",
  "exp": 4102444800,
  "iat": 1461018939
}

v4_delegated_data = {
  "mrid": "TWVtYmVyc2hpcB81NzE5MjMyMzY5MTMxNTIw",
  "mem": 5719232369131520,
  "roles": [
    "super-admin|super-admin",
    "super-admin|restricted-admin",
    "super-admin|server-admin-rapid-response",
    "remote-console|remote-console-admin",
    "super-admin|server-admin-software-support-engineer",
    "super-admin|server-admin-customer-success",
    "super-admin|server-admin-viewer",
    "super-admin|release-management",
    "remote-console|remote-console-viewer"
  ],
  "scope": "sox|r sox|w",
  "licenses": [
    "section16User",
    "section16"
  ],
  "scp": [
    "sox|r",
    "sox|w"
  ],
  "acc": 5688422018580480,
  "par": "a3eb82dbe26f4f29bfecb370504c1b70",
  "ver": 4,
  "aud": "test-client-id",
  "gnt": "urn:ietf:params:oauth:grant-type:jwt-bearer",
  "cid": "test-client-id",
  "iss": "https://wk-dev.wdesk.org",
  "wsk": 0,
  "arid": "QWNjb3VudB81Njg4NDIyMDE4NTgwNDgw",
  "jti": "307c25fc798041118060d8eaa2accf21",
  "usr": 5728415315394560,
  "exp": 4102444800,
  "iat": 1461019189,
  "urid": "V0ZVc2VyHzU3Mjg0MTUzMTUzOTQ1NjA"
}


# Not an actual token, but left in for testing backwards compatibility.
valid_token_data = {
  "acc": 5688422018580480,
  "iss": "https://wk-dev.wdesk.org",
  "usr": 5728415315394560,
  "exp": 4102444800,
  "cid": "test-client-id",
  "mem": 5719232369131520,
  "wsk": 1,
  "scp": [
    "sox|r",
    "sox|w"
  ]
}

expired_token_data = valid_token_data.copy()
expired_token_data['exp'] = expired_exp


##################
# Token generation
##################

rsa_key = Crypto.PublicKey.RSA.construct(key_tuple)
pub = rsa_key.publickey()

token = jwt.encode(v5_data, rsa_key, 'RS512')

print token
